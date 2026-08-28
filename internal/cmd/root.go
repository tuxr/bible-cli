package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/render"
	"github.com/tuxr/bible-cli/internal/theme"
)

const (
	exitOK       = 0
	exitUsage    = 1
	exitNotFound = 2
	exitSystem   = 3
)

// book then chapter number (optional verse/range). Leading book numbers like "1 John" are allowed.
var hasChapterRE = regexp.MustCompile(`(?i)^(?:\d+\s+)?[A-Za-z]+(?:\s+[A-Za-z]+)*\s+\d+`)

type options struct {
	translation string
	jsonOut     bool
	apiURL      string
	color       string
	redLetter   bool
	theme       string
}

type usageError struct {
	msg string
}

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// NewRoot builds the bible command tree (lookup on the root; `read` is an alias).
func NewRoot() *cobra.Command {
	o := &options{}
	root := &cobra.Command{
		Use:           "bible [reference]",
		Short:         "Look up Bible verses from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return o.lookup(cmd, args)
		},
	}
	o.bindFlags(root)

	read := &cobra.Command{
		Use:   "read [reference]",
		Short: "Look up a verse or chapter",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usagef("pass a chapter")
			}
			return o.lookup(cmd, args)
		},
	}
	root.AddCommand(read)
	root.CompletionOptions.DisableDefaultCmd = true
	return root
}

func (o *options) bindFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.StringVarP(&o.translation, "translation", "t", "web", "translation id")
	f.BoolVar(&o.jsonOut, "json", false, "print JSON")
	f.StringVar(&o.apiURL, "api-url", "", "bible-api base URL")
	f.StringVar(&o.color, "color", "auto", "color output: auto|always|never")
	f.BoolVar(&o.redLetter, "red-letter", true, "color words of Jesus")
	f.StringVar(&o.theme, "theme", "auto", "theme: dark|light|auto")
}

// Execute runs the CLI with process args and stdio.
func Execute() int {
	return ExecuteWith(os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteWith runs the CLI with explicit args and writers. A nil args slice is
// treated as no arguments (not os.Args) so tests cannot leak into flag parsing.
func ExecuteWith(args []string, stdout, stderr io.Writer) int {
	if args == nil {
		args = []string{}
	}
	root := NewRoot()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	return exitOK
}

func exitCode(err error) int {
	var ue *usageError
	if errors.As(err, &ue) {
		return exitUsage
	}
	var ae *api.APIError
	if errors.As(err, &ae) {
		if ae.Kind == "not-found" {
			return exitNotFound
		}
		return exitSystem
	}
	return exitUsage
}

func (o *options) lookup(cmd *cobra.Command, args []string) error {
	ref := strings.Join(args, " ")
	if !hasChapter(ref) {
		return usagef("pass a chapter")
	}
	switch strings.ToLower(strings.TrimSpace(o.color)) {
	case "auto", "always", "never":
	default:
		return usagef("invalid color %q (auto|always|never)", o.color)
	}
	pal, err := theme.Lookup(o.theme)
	if err != nil {
		return usagef("%s", err.Error())
	}

	out := cmd.OutOrStdout()
	client := api.New(o.apiURL)
	resp, err := client.GetVerses(cmd.Context(), ref, o.translation)
	if err != nil {
		return err
	}
	if o.jsonOut {
		return render.WriteJSON(out, resp)
	}

	header := resp.Reference
	if id := resp.Translation.ID; id != "" {
		header = fmt.Sprintf("%s (%s)", resp.Reference, strings.ToUpper(id))
	}
	block := render.LookupBlock(header, resp.Verses, render.Options{
		Color:     o.colorEnabled(out),
		RedLetter: o.redLetter,
		Palette:   pal,
	})
	_, err = fmt.Fprintln(out, block)
	return err
}

func (o *options) colorEnabled(w io.Writer) bool {
	switch strings.ToLower(strings.TrimSpace(o.color)) {
	case "always":
		return true
	case "never":
		return false
	default:
		return isTTY(w)
	}
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func hasChapter(ref string) bool {
	return hasChapterRE.MatchString(strings.TrimSpace(ref))
}

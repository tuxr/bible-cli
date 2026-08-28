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
	"github.com/tuxr/bible-cli/internal/config"
	"github.com/tuxr/bible-cli/internal/render"
	"github.com/tuxr/bible-cli/internal/theme"
)

const (
	exitOK       = 0
	exitUsage    = 1
	exitNotFound = 2
	exitSystem   = 3
)

// Version is the bible-cli version printed by `bible version` and sent as User-Agent.
// Wired from main (GoReleaser -ldflags); defaults to "dev".
var Version = "dev"

var reservedRefs = map[string]struct{}{
	"config": {},
	"tui":    {},
}

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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateColor(o.color); err != nil {
				return err
			}
			if skipConfigLoad(cmd) {
				return nil
			}
			return o.applyConfig(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if _, reserved := reservedRefs[strings.ToLower(args[0])]; reserved {
				return usagef("%s is a reserved command", args[0])
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
	o.addCatalogCommands(root)
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print bible-cli version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), Version)
			return err
		},
	})
	return root
}

func (o *options) bindFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.StringVarP(&o.translation, "translation", "t", config.DefaultTranslation, "translation id")
	f.BoolVar(&o.jsonOut, "json", false, "print JSON")
	f.StringVar(&o.apiURL, "api-url", "", "bible-api base URL")
	f.StringVar(&o.color, "color", "auto", "color output: auto|always|never")
	f.BoolVar(&o.redLetter, "red-letter", config.DefaultRedLetter, "color words of Jesus")
	f.StringVar(&o.theme, "theme", config.DefaultTheme, "theme: dark|light|auto")
}

func (o *options) applyConfig(cmd *cobra.Command) error {
	if err := validateColor(o.color); err != nil {
		return err
	}
	fs := cmd.Flags()
	cfg, err := config.Load(config.Flags{
		Translation:    o.translation,
		Theme:          o.theme,
		APIURL:         o.apiURL,
		RedLetter:      o.redLetter,
		HasTranslation: fs.Changed("translation"),
		HasTheme:       fs.Changed("theme"),
		HasAPIURL:      fs.Changed("api-url"),
		HasRedLetter:   fs.Changed("red-letter"),
	})
	if err != nil {
		return err
	}
	o.translation = cfg.Translation
	o.theme = cfg.Theme
	o.apiURL = cfg.APIURL
	o.redLetter = cfg.RedLetter
	return nil
}

// Execute runs the CLI with process args and stdio.
func Execute(version string) int {
	if v := strings.TrimSpace(version); v != "" {
		Version = v
	}
	return ExecuteWith(os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteWith runs the CLI with explicit args and writers. A nil args slice is
// treated as no arguments (not os.Args) so tests cannot leak into flag parsing.
func ExecuteWith(args []string, stdout, stderr io.Writer) int {
	if args == nil {
		args = []string{}
	}
	api.Version = Version
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
	if _, err := o.palette(); err != nil {
		return err
	}
	client := api.New(o.apiURL)
	resp, err := client.GetVerses(cmd.Context(), ref, o.translation)
	if err != nil {
		return err
	}
	return o.writeVerseResponse(cmd, resp)
}

func (o *options) writeVerseResponse(cmd *cobra.Command, resp *api.VerseResponse) error {
	pal, err := o.palette()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if o.jsonOut {
		return render.WriteJSON(out, resp)
	}

	opts := render.Options{
		Color:     o.colorEnabled(cmd, out),
		RedLetter: o.redLetter,
		Palette:   pal,
	}
	if !writerIsTTY(out) {
		opts.Color = false
		_, err = fmt.Fprintln(out, render.PipedLookup(resp.Reference, resp.Verses, opts))
		return err
	}
	header := resp.Reference
	if id := resp.Translation.ID; id != "" {
		header = fmt.Sprintf("%s (%s)", resp.Reference, strings.ToUpper(id))
	}
	block := render.LookupBlock(header, resp.Verses, opts)
	_, err = fmt.Fprintln(out, block)
	return err
}

func skipConfigLoad(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "completion", "version":
			return true
		}
	}
	return false
}

func validateColor(v string) error {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "auto", "always", "never":
		return nil
	default:
		return usagef("invalid color %q (auto|always|never)", v)
	}
}

func colorFlagChanged(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup("color"); f != nil && f.Changed {
			return true
		}
	}
	return false
}

func (o *options) palette() (theme.Palette, error) {
	if err := validateColor(o.color); err != nil {
		return theme.Palette{}, err
	}
	pal, err := theme.Lookup(o.theme)
	if err != nil {
		return theme.Palette{}, usagef("%s", err.Error())
	}
	return pal, nil
}

func (o *options) colorEnabled(cmd *cobra.Command, w io.Writer) bool {
	switch o.resolvedColor(cmd) {
	case "always":
		return true
	case "never":
		return false
	default:
		return writerIsTTY(w)
	}
}

func (o *options) resolvedColor(cmd *cobra.Command) string {
	mode := strings.ToLower(strings.TrimSpace(o.color))
	if colorFlagChanged(cmd) {
		return mode
	}
	if os.Getenv("NO_COLOR") != "" {
		return "never"
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return "always"
	}
	return mode
}

// writerIsTTY is swapped in tests to simulate a TTY stdout.
var writerIsTTY = isTTY

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

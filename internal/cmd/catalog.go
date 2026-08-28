package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/render"
)

func (o *options) addCatalogCommands(root *cobra.Command) {
	var searchBook, searchTestament string
	var searchLimit int
	search := &cobra.Command{
		Use:   "search [query]",
		Short: "Search verses",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.search(cmd, args, searchBook, searchTestament, searchLimit)
		},
	}
	search.Flags().StringVar(&searchBook, "book", "", "limit to book id (e.g. JHN)")
	search.Flags().StringVar(&searchTestament, "testament", "", "filter: OT|NT|AP")
	search.Flags().IntVar(&searchLimit, "limit", 0, "max results")

	translations := &cobra.Command{
		Use:   "translations",
		Short: "List available translations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.translations(cmd)
		},
	}

	var booksTestament string
	books := &cobra.Command{
		Use:   "books",
		Short: "List books",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.books(cmd, booksTestament)
		},
	}
	books.Flags().StringVar(&booksTestament, "testament", "", "filter: OT|NT|AP")

	var randomBook, randomTestament string
	random := &cobra.Command{
		Use:   "random",
		Short: "Print a random verse",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.random(cmd, randomBook, randomTestament)
		},
	}
	random.Flags().StringVar(&randomBook, "book", "", "limit to book id (e.g. PSA)")
	random.Flags().StringVar(&randomTestament, "testament", "", "limit to OT|NT|AP")

	root.AddCommand(search, translations, books, random)
}

func (o *options) search(cmd *cobra.Command, args []string, book, testament string, limit int) error {
	q := strings.TrimSpace(strings.Join(args, " "))
	if q == "" {
		return usagef("pass a search query")
	}
	ts, err := normalizeTestament(testament)
	if err != nil {
		return err
	}
	if limit < 0 {
		return usagef("invalid limit %d (must be >= 0)", limit)
	}
	out := cmd.OutOrStdout()
	client := api.New(o.apiURL)
	resp, err := client.Search(cmd.Context(), api.SearchQuery{
		Q:           q,
		Translation: o.translation,
		Book:        strings.TrimSpace(book),
		Testament:   ts,
		Limit:       limit,
	})
	if err != nil {
		return err
	}
	if o.catalogJSON(out) {
		return render.WriteJSON(out, resp)
	}
	_, err = fmt.Fprintln(out, render.SearchHits(resp.Results))
	return err
}

func (o *options) translations(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	client := api.New(o.apiURL)
	list, err := client.Translations(cmd.Context())
	if err != nil {
		return err
	}
	list = preferWebFirst(list)
	if o.catalogJSON(out) {
		return render.WriteJSON(out, list)
	}
	_, err = fmt.Fprintln(out, render.Translations(list))
	return err
}

func (o *options) books(cmd *cobra.Command, testament string) error {
	ts, err := normalizeTestament(testament)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	client := api.New(o.apiURL)
	list, err := client.Books(cmd.Context(), ts)
	if err != nil {
		return err
	}
	if o.catalogJSON(out) {
		return render.WriteJSON(out, list)
	}
	_, err = fmt.Fprintln(out, render.Books(list))
	return err
}

func (o *options) random(cmd *cobra.Command, book, testament string) error {
	ts, err := normalizeTestament(testament)
	if err != nil {
		return err
	}
	if _, err := o.palette(); err != nil {
		return err
	}
	client := api.New(o.apiURL)
	resp, err := client.Random(cmd.Context(), o.translation, strings.TrimSpace(book), ts)
	if err != nil {
		return err
	}
	return o.writeVerseResponse(cmd, resp)
}

func (o *options) catalogJSON(w io.Writer) bool {
	return o.jsonOut || !writerIsTTY(w)
}

func preferWebFirst(list []api.Translation) []api.Translation {
	webIdx := -1
	for i, tr := range list {
		if strings.EqualFold(tr.ID, "web") {
			webIdx = i
			break
		}
	}
	if webIdx <= 0 {
		return list
	}
	out := make([]api.Translation, 0, len(list))
	out = append(out, list[webIdx])
	out = append(out, list[:webIdx]...)
	out = append(out, list[webIdx+1:]...)
	return out
}

func normalizeTestament(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	switch strings.ToUpper(v) {
	case "OT", "NT", "AP":
		return strings.ToUpper(v), nil
	default:
		return "", usagef("invalid testament %q (OT|NT|AP)", v)
	}
}

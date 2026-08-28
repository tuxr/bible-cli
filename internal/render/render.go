package render

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/theme"
)

// Options controls verse rendering.
type Options struct {
	Color     bool
	RedLetter bool
	Palette   theme.Palette
}

func jesusStyle(p theme.Palette) lipgloss.Style {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r.NewStyle().Foreground(lipgloss.Color(p.RedLetter))
}

func verseText(v api.Verse, opts Options) string {
	if len(v.Segments) == 0 {
		return v.Text
	}
	paint := opts.Color && opts.RedLetter
	var red lipgloss.Style
	if paint {
		red = jesusStyle(opts.Palette)
	}
	var b strings.Builder
	for _, seg := range v.Segments {
		if paint && seg.Speaker == "jesus" {
			b.WriteString(red.Render(seg.Text))
			continue
		}
		b.WriteString(seg.Text)
	}
	return b.String()
}

// VerseLine formats a single verse, coloring only speaker=="jesus" segments
// when Color and RedLetter are both set.
func VerseLine(v api.Verse, opts Options) string {
	return strconv.Itoa(v.Verse) + "  " + verseText(v, opts)
}

// LookupBlock renders a header followed by verse lines.
func LookupBlock(header string, verses []api.Verse, opts Options) string {
	var b strings.Builder
	if header != "" {
		b.WriteString(header)
		if len(verses) > 0 {
			b.WriteByte('\n')
		}
	}
	for i, v := range verses {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(VerseLine(v, opts))
	}
	return b.String()
}

// WriteJSON writes v as JSON. It does not add a redundant top-level text field.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

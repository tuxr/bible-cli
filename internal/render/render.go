package render

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/theme"
)

// trueColor is termenv.TrueColor (iota 0). Passed as an untyped int so
// render does not import termenv and go.mod can keep it indirect.
const trueColor = 0

// Options controls verse rendering.
type Options struct {
	Color     bool
	RedLetter bool
	Palette   theme.Palette
}

func jesusStyle(p theme.Palette) lipgloss.Style {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(trueColor)
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

// PipedLookup is the pipe layout: canonical reference, then either a single
// verse as text or verse-number<TAB>text lines for multiple verses.
func PipedLookup(reference string, verses []api.Verse, opts Options) string {
	var b strings.Builder
	b.WriteString(reference)
	switch len(verses) {
	case 0:
		return b.String()
	case 1:
		b.WriteByte('\n')
		b.WriteString(verseText(verses[0], opts))
	default:
		for _, v := range verses {
			b.WriteByte('\n')
			b.WriteString(strconv.Itoa(v.Verse))
			b.WriteByte('	')
			b.WriteString(verseText(v, opts))
		}
	}
	return b.String()
}

// WriteJSON writes v as JSON. It does not add a redundant top-level text field.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

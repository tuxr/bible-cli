package render

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/theme"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func paintRed(text string, p theme.Palette) string {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r.NewStyle().Foreground(lipgloss.Color(p.RedLetter)).Render(text)
}

func john33() api.Verse {
	return api.Verse{
		Book:     "JHN",
		BookName: "John",
		Chapter:  3,
		Verse:    3,
		Text:     "Jesus answered him, “Most certainly I tell you, unless one is born anew, he can’t see God’s Kingdom.”",
		Segments: []api.Segment{
			{Text: "Jesus answered him,", Speaker: "narrator"},
			{Text: " “Most certainly I tell you, unless one is born anew, he can’t see God’s Kingdom.”", Speaker: "jesus"},
		},
	}
}

func TestVerseLineRedLetterDark(t *testing.T) {
	v := john33()
	narrator := v.Segments[0].Text
	jesus := v.Segments[1].Text
	out := VerseLine(v, Options{Color: true, RedLetter: true, Palette: theme.Dark})
	if !strings.Contains(out, "\x1b") {
		t.Fatalf("expected ANSI, got %q", out)
	}
	stripped := stripANSI(out)
	if !strings.Contains(stripped, narrator) {
		t.Fatalf("stripped missing narrator %q in %q", narrator, stripped)
	}
	if !strings.Contains(stripped, jesus) {
		t.Fatalf("stripped missing jesus %q in %q", jesus, stripped)
	}
	wrapped := paintRed(jesus, theme.Dark)
	if !strings.Contains(out, wrapped) {
		t.Fatalf("jesus portion not wrapped, got %q want substring %q", out, wrapped)
	}
	if !strings.Contains(out, narrator) {
		t.Fatal("narrator not present as raw (unstyled) text")
	}
	if strings.Contains(out, paintRed(narrator, theme.Dark)) {
		t.Fatal("narrator portion is red")
	}
}

func TestVerseLineColorWithoutRedLetter(t *testing.T) {
	v := john33()
	out := VerseLine(v, Options{Color: true, RedLetter: false, Palette: theme.Dark})
	stripped := stripANSI(out)
	if !strings.Contains(stripped, v.Segments[0].Text) || !strings.Contains(stripped, v.Segments[1].Text) {
		t.Fatalf("missing full text in %q", stripped)
	}
	if strings.Contains(out, paintRed(v.Segments[1].Text, theme.Dark)) {
		t.Fatalf("red-letter codes present when RedLetter=false: %q", out)
	}
}

func TestVerseLineNilSegmentsFallsBackToText(t *testing.T) {
	v := api.Verse{
		Verse: 3,
		Text:  "Jesus answered him, “Most certainly I tell you, unless one is born anew, he can’t see God’s Kingdom.”",
	}
	out := VerseLine(v, Options{Color: true, RedLetter: true, Palette: theme.Dark})
	if !strings.Contains(out, v.Text) {
		t.Fatalf("fallback missing Verse.Text: %q", out)
	}
	if strings.Contains(out, "\x1b") {
		t.Fatalf("nil segments must not color from verse text: %q", out)
	}
}

func TestVerseLineColorFalseNoANSI(t *testing.T) {
	out := VerseLine(john33(), Options{Color: false, RedLetter: true, Palette: theme.Dark})
	if strings.Contains(out, "\x1b") {
		t.Fatalf("ANSI present when Color=false: %q", out)
	}
}

func TestLookupBlockHeaderAndVerses(t *testing.T) {
	v4 := api.Verse{Verse: 4, Text: "Nicodemus said to him, “How can a man be born when he is old?”"}
	out := LookupBlock("John 3 (WEB)", []api.Verse{john33(), v4}, Options{Color: false})
	if strings.Contains(out, "\x1b") {
		t.Fatalf("ANSI present: %q", out)
	}
	if !strings.Contains(out, "John 3 (WEB)") {
		t.Fatalf("missing header in %q", out)
	}
	if !strings.Contains(out, john33().Segments[0].Text) || !strings.Contains(out, john33().Segments[1].Text) {
		t.Fatalf("missing John 3:3 text in %q", out)
	}
	if !strings.Contains(out, v4.Text) {
		t.Fatalf("missing John 3:4 text in %q", out)
	}
}

func TestWriteJSONVerseResponseOmitsTopLevelTextKeepsSegments(t *testing.T) {
	resp := api.VerseResponse{
		Reference: "John 3:3",
		Translation: api.Translation{
			ID:       "web",
			Name:     "World English Bible",
			Language: "en",
		},
		Verses: []api.Verse{john33()},
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, resp); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["text"]; ok {
		t.Fatalf("redundant top-level text present: %s", buf.Bytes())
	}
	verses, ok := m["verses"].([]any)
	if !ok || len(verses) != 1 {
		t.Fatalf("verses = %#v", m["verses"])
	}
	v0, ok := verses[0].(map[string]any)
	if !ok {
		t.Fatalf("verse = %#v", verses[0])
	}
	segs, ok := v0["segments"].([]any)
	if !ok || len(segs) != 2 {
		t.Fatalf("segments = %#v", v0["segments"])
	}
	s0, _ := segs[0].(map[string]any)
	s1, _ := segs[1].(map[string]any)
	if s0["speaker"] != "narrator" || s1["speaker"] != "jesus" {
		t.Fatalf("speakers = %#v %#v", s0["speaker"], s1["speaker"])
	}
	if s0["text"] != resp.Verses[0].Segments[0].Text || s1["text"] != resp.Verses[0].Segments[1].Text {
		t.Fatalf("segment text mismatch: %#v", segs)
	}
}

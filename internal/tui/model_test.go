package tui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/render"
	"github.com/tuxr/bible-cli/internal/theme"
)

type apiLog struct {
	paths   []string
	queries []string
}

type saver struct {
	book    string
	chapter int
	n       int
	err     error
}

func (s *saver) save(book string, chapter int) error {
	s.n++
	s.book = book
	s.chapter = chapter
	return s.err
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newTUIServer(t *testing.T) (*httptest.Server, *apiLog) {
	t.Helper()
	log := &apiLog{}
	john3 := fixture(t, "chapter_john_3_segments.json")
	verse := fixture(t, "verse_john_3_16_segments.json")
	books := fixture(t, "books_nt.json")
	translations := fixture(t, "translations.json")
	searchLove := fixture(t, "search_love.json")

	jhn1 := mustChapter(t, "JHN", "John", 1, []api.Verse{{Verse: 1, Text: "In the beginning was the Word, and the Word was with God, and the Word was God."}},
		&api.NavRef{Book: "LUK", Chapter: 24}, &api.NavRef{Book: "JHN", Chapter: 2})
	jhn2 := mustChapter(t, "JHN", "John", 2, []api.Verse{{Verse: 1, Text: "On the third day there was a wedding in Cana of Galilee."}},
		&api.NavRef{Book: "JHN", Chapter: 1}, &api.NavRef{Book: "JHN", Chapter: 3})
	jhn4 := mustChapter(t, "JHN", "John", 4, []api.Verse{{Verse: 1, Text: "Therefore when the Lord knew that the Pharisees had heard."}},
		&api.NavRef{Book: "JHN", Chapter: 3}, &api.NavRef{Book: "JHN", Chapter: 5})
	gen1 := mustChapter(t, "GEN", "Genesis", 1, []api.Verse{{Verse: 1, Text: "In the beginning God created the heavens and the earth."}},
		nil, &api.NavRef{Book: "GEN", Chapter: 2})
	gen22 := mustChapter(t, "GEN", "Genesis", 22, []api.Verse{
		{Verse: 1, Text: "After these things, God tested Abraham."},
		{Verse: 2, Text: "He said, Now take your son, your only son, Isaac, whom you love."},
	}, &api.NavRef{Book: "GEN", Chapter: 21}, &api.NavRef{Book: "GEN", Chapter: 23})
	rev22 := mustChapter(t, "REV", "Revelation", 22, []api.Verse{{Verse: 1, Text: "He showed me a river of water of life."}},
		&api.NavRef{Book: "REV", Chapter: 21}, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.paths = append(log.paths, r.URL.Path)
		log.queries = append(log.queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		trans := r.URL.Query().Get("translation")
		if trans == "wlc" && strings.HasPrefix(r.URL.Path, "/v1/chapters/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Not found"}`))
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/JHN/3"):
			_, _ = w.Write(withTranslationJSON(t, john3, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/JHN/1"):
			writeJSON(w, withTranslation(jhn1, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/JHN/2"):
			writeJSON(w, withTranslation(jhn2, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/JHN/4"):
			writeJSON(w, withTranslation(jhn4, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/GEN/1"):
			writeJSON(w, withTranslation(gen1, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/GEN/22"):
			writeJSON(w, withTranslation(gen22, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/chapters/REV/22"):
			writeJSON(w, withTranslation(rev22, trans))
		case strings.HasPrefix(r.URL.Path, "/v1/verses/"):
			_, _ = w.Write(verse)
		case r.URL.Path == "/v1/books":
			_, _ = w.Write(books)
		case r.URL.Path == "/v1/translations":
			_, _ = w.Write(translations)
		case r.URL.Path == "/v1/search":
			_, _ = w.Write(searchLove)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

func mustChapter(t *testing.T, id, name string, n int, verses []api.Verse, prev, next *api.NavRef) *api.ChapterResponse {
	t.Helper()
	return &api.ChapterResponse{
		Book:        api.ChapterBook{ID: id, Name: name, Testament: "OT"},
		Chapter:     n,
		Translation: api.Translation{ID: "web", Name: "World English Bible"},
		VerseCount:  len(verses),
		Verses:      verses,
		Navigation:  api.Navigation{Previous: prev, Next: next},
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func withTranslation(ch *api.ChapterResponse, trans string) *api.ChapterResponse {
	if ch == nil || trans == "" || trans == "web" {
		return ch
	}
	cp := *ch
	cp.Translation.ID = trans
	return &cp
}

func withTranslationJSON(t *testing.T, raw []byte, trans string) []byte {
	t.Helper()
	if trans == "" || trans == "web" {
		return raw
	}
	var ch api.ChapterResponse
	if err := json.Unmarshal(raw, &ch); err != nil {
		t.Fatal(err)
	}
	ch.Translation.ID = trans
	b, err := json.Marshal(ch)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func asModel(t *testing.T, tm tea.Model) Model {
	t.Helper()
	m, ok := tm.(Model)
	if !ok {
		t.Fatalf("model type %T", tm)
	}
	return m
}

func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for i := 0; cmd != nil && i < 8; i++ {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			return m
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = asModel(t, next)
	}
	return m
}

func load(t *testing.T, opts Options) Model {
	t.Helper()
	m := New(opts)
	return drain(t, m, m.Init())
}

func key(s string) tea.KeyMsg {
	switch s {
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc", "escape":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func clientOpts(t *testing.T, srv *httptest.Server, extra Options) Options {
	t.Helper()
	extra.Client = api.New(srv.URL)
	if extra.Palette.Fg == "" {
		extra.Palette = theme.Dark
	}
	if extra.Translation == "" {
		extra.Translation = "web"
	}
	return extra
}

func TestInitResumeLoadsChapter(t *testing.T) {
	srv, log := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	if m.state != stateReady {
		t.Fatalf("state = %d, want ready; status=%q", m.state, m.status)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 {
		t.Fatalf("chapter = %+v", m.chapter)
	}
	if !strings.Contains(strings.Join(log.paths, ","), "/v1/chapters/JHN/3") {
		t.Fatalf("paths = %v, want JHN/3", log.paths)
	}
	if sv.n != 1 || sv.book != "JHN" || sv.chapter != 3 {
		t.Fatalf("save = %+v", sv)
	}
}

func TestInitDefaultGEN1(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{}))
	if m.chapter == nil || m.chapter.Book.ID != "GEN" || m.chapter.Chapter != 1 {
		t.Fatalf("chapter = %+v", m.chapter)
	}
	if !strings.Contains(strings.Join(log.paths, ","), "/v1/chapters/GEN/1") {
		t.Fatalf("paths = %v", log.paths)
	}
}

func TestInitInvalidResumeFallsBackGEN1(t *testing.T) {
	srv, log := newTUIServer(t)
	tests := []Options{
		{ResumeBook: "", ResumeChapter: 3},
		{ResumeBook: "JHN", ResumeChapter: 0},
	}
	for _, opts := range tests {
		log.paths = nil
		m := load(t, clientOpts(t, srv, opts))
		if m.chapter == nil || m.chapter.Book.ID != "GEN" {
			t.Fatalf("opts=%+v chapter=%+v", opts, m.chapter)
		}
		joined := strings.Join(log.paths, ",")
		if strings.Contains(joined, "/v1/chapters/JHN/") {
			t.Fatalf("used invalid resume: %v", log.paths)
		}
	}
}

func TestInitUnknownOrOutOfRangeResumeFallsBackGEN1(t *testing.T) {
	srv, log := newTUIServer(t)
	tests := []struct {
		name    string
		book    string
		chapter int
		tried   string
	}{
		{name: "unknown book", book: "NOPE", chapter: 1, tried: "/v1/chapters/NOPE/1"},
		{name: "out of range", book: "JHN", chapter: 99, tried: "/v1/chapters/JHN/99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.paths = nil
			m := load(t, clientOpts(t, srv, Options{ResumeBook: tt.book, ResumeChapter: tt.chapter}))
			if m.state != stateReady {
				t.Fatalf("state = %d, want ready; status=%q err=%v", m.state, m.status, m.err)
			}
			if m.chapter == nil || m.chapter.Book.ID != "GEN" || m.chapter.Chapter != 1 {
				t.Fatalf("chapter = %+v", m.chapter)
			}
			joined := strings.Join(log.paths, ",")
			if !strings.Contains(joined, tt.tried) {
				t.Fatalf("expected resume fetch %s, paths=%v", tt.tried, log.paths)
			}
			if !strings.Contains(joined, "/v1/chapters/GEN/1") {
				t.Fatalf("expected GEN/1 fallback, paths=%v", log.paths)
			}
		})
	}
}

func TestInitExplicitRefFailureStaysError(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{Ref: "NOPE"}))
	if m.state != stateError {
		t.Fatalf("state = %d, want error; status=%q", m.state, m.status)
	}
	joined := strings.Join(log.paths, ",")
	if strings.Contains(joined, "/v1/chapters/GEN/1") {
		t.Fatalf("explicit ref must not fall back to GEN/1: %v", log.paths)
	}
}

func TestInitExplicitVerseRefOverridesResume(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{
		Ref:           "John 3:16",
		ResumeBook:    "GEN",
		ResumeChapter: 1,
	}))
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 {
		t.Fatalf("chapter = %+v", m.chapter)
	}
	joined := strings.Join(log.paths, ",")
	if !strings.Contains(joined, "/v1/verses/") {
		t.Fatalf("expected verse resolve, paths=%v", log.paths)
	}
	if !strings.Contains(joined, "/v1/chapters/JHN/3") {
		t.Fatalf("expected chapter fetch, paths=%v", log.paths)
	}
	if strings.Contains(joined, "/v1/chapters/GEN/1") {
		t.Fatalf("resume must not win: %v", log.paths)
	}
}

func TestInitBookOnlyRef(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{Ref: "Jn"}))
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 1 {
		t.Fatalf("chapter = %+v status=%q paths=%v", m.chapter, m.status, log.paths)
	}
	joined := strings.Join(log.paths, ",")
	if !strings.Contains(joined, "/v1/books") {
		t.Fatalf("expected books lookup, paths=%v", log.paths)
	}
	if !strings.Contains(joined, "/v1/chapters/JHN/1") {
		t.Fatalf("expected chapter 1, paths=%v", log.paths)
	}
}

func TestChapterFetchSendsSegmentsAndTranslation(t *testing.T) {
	srv, log := newTUIServer(t)
	_ = load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, Translation: "web"}))
	if len(log.queries) == 0 {
		t.Fatal("no queries")
	}
	q := log.queries[0]
	if !strings.Contains(q, "segments=1") {
		t.Fatalf("query = %q, want segments=1", q)
	}
	if !strings.Contains(q, "translation=web") {
		t.Fatalf("query = %q, want translation=web", q)
	}
}

func TestNextPrevFollowNavigation(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, cmd := m.Update(key("n"))
	m = drain(t, asModel(t, next), cmd)
	if m.chapter.Chapter != 4 || m.chapter.Book.ID != "JHN" {
		t.Fatalf("after n: %+v", m.chapter)
	}
	next, cmd = m.Update(key("left"))
	m = drain(t, asModel(t, next), cmd)
	if m.chapter.Chapter != 3 {
		t.Fatalf("after left from 4: %+v", m.chapter)
	}
	next, cmd = m.Update(key("p"))
	m = drain(t, asModel(t, next), cmd)
	if m.chapter.Chapter != 2 {
		t.Fatalf("after p: %+v", m.chapter)
	}
	next, cmd = m.Update(key("right"))
	m = drain(t, asModel(t, next), cmd)
	if m.chapter.Chapter != 3 {
		t.Fatalf("after right: %+v", m.chapter)
	}
}

func TestNilNavigationStaysPut(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	before := len(log.paths)
	next, cmd := m.Update(key("p"))
	m = asModel(t, next)
	if cmd != nil {
		t.Fatal("expected no fetch at start of canon")
	}
	if m.chapter.Book.ID != "GEN" || m.chapter.Chapter != 1 {
		t.Fatalf("moved: %+v", m.chapter)
	}
	if !strings.Contains(m.status, "start of canon") {
		t.Fatalf("status = %q", m.status)
	}
	if len(log.paths) != before {
		t.Fatalf("extra requests: %v", log.paths[before:])
	}

	m = load(t, clientOpts(t, srv, Options{ResumeBook: "REV", ResumeChapter: 22}))
	before = len(log.paths)
	next, cmd = m.Update(key("n"))
	m = asModel(t, next)
	if cmd != nil {
		t.Fatal("expected no fetch at end of canon")
	}
	if m.chapter.Book.ID != "REV" || m.chapter.Chapter != 22 {
		t.Fatalf("moved: %+v", m.chapter)
	}
	if !strings.Contains(m.status, "end of canon") {
		t.Fatalf("status = %q", m.status)
	}
	if len(log.paths) != before {
		t.Fatalf("extra requests: %v", log.paths[before:])
	}
}

func TestCursorJK(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	if m.cursor != 0 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	next, _ := m.Update(key("j"))
	m = asModel(t, next)
	if m.cursor != 1 {
		t.Fatalf("after j cursor = %d", m.cursor)
	}
	next, _ = m.Update(key("down"))
	m = asModel(t, next)
	if m.cursor != 2 {
		t.Fatalf("after down cursor = %d", m.cursor)
	}
	next, _ = m.Update(key("k"))
	m = asModel(t, next)
	if m.cursor != 1 {
		t.Fatalf("after k cursor = %d", m.cursor)
	}
	next, _ = m.Update(key("up"))
	m = asModel(t, next)
	if m.cursor != 0 {
		t.Fatalf("after up cursor = %d", m.cursor)
	}
	next, _ = m.Update(key("k"))
	m = asModel(t, next)
	if m.cursor != 0 {
		t.Fatalf("k at top should stay 0, got %d", m.cursor)
	}
	for i := 0; i < 100; i++ {
		next, _ = m.Update(key("j"))
		m = asModel(t, next)
	}
	if m.cursor != len(m.chapter.Verses)-1 {
		t.Fatalf("cursor = %d, want last %d", m.cursor, len(m.chapter.Verses)-1)
	}
}

func TestHelpToggle(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	if m.help {
		t.Fatal("help on at start")
	}
	next, _ := m.Update(key("?"))
	m = asModel(t, next)
	if !m.help {
		t.Fatal("help not toggled on")
	}
	view := m.View()
	if !strings.Contains(view, "toggle help") {
		t.Fatalf("help overlay missing:\n%s", view)
	}
	if !strings.Contains(view, "q quit") {
		t.Fatalf("footer missing in help:\n%s", view)
	}
	next, _ = m.Update(key("?"))
	m = asModel(t, next)
	if m.help {
		t.Fatal("help not toggled off")
	}
}

func TestQuit(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q must return a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q cmd produced %T, want QuitMsg", msg)
	}
}

func TestTinyWidthNoPanic(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, Color: true, RedLetter: true}))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 10, Height: 4})
	m = asModel(t, next)
	view := m.View()
	if view == "" {
		t.Fatal("empty view")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	m = asModel(t, next)
	_ = m.View()
}

func TestRedLetterInView(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{
		ResumeBook:    "JHN",
		ResumeChapter: 3,
		Color:         true,
		RedLetter:     true,
		Palette:       theme.Dark,
	}))
	view := m.View()
	if !strings.Contains(view, "bible") {
		t.Fatalf("masthead missing:\n%s", view)
	}
	if !strings.Contains(view, "John 3") || !strings.Contains(view, "WEB") {
		t.Fatalf("location missing:\n%s", view)
	}
	v := m.chapter.Verses[2] // John 3:3 has jesus segment
	want := render.VerseLine(v, render.Options{Color: true, RedLetter: true, Palette: theme.Dark})
	jesus := v.Segments[1].Text
	if !strings.Contains(view, jesus) && !strings.Contains(view, "Most certainly I tell you") {
		t.Fatalf("jesus text missing:\n%s", view)
	}
	if !strings.Contains(want, "\x1b") {
		t.Fatal("VerseLine should paint jesus")
	}
	if !strings.Contains(view, "\x1b") {
		t.Fatalf("expected ANSI red-letter in view:\n%q", view)
	}
}

func TestResumeSaveAfterSuccess(t *testing.T) {
	srv, _ := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	if sv.n != 1 {
		t.Fatalf("saves = %d", sv.n)
	}
	next, cmd := m.Update(key("n"))
	_ = drain(t, asModel(t, next), cmd)
	if sv.n != 2 || sv.book != "JHN" || sv.chapter != 4 {
		t.Fatalf("save after n = %+v", sv)
	}
}

func TestResumeSaveFailureNonfatal(t *testing.T) {
	srv, _ := newTUIServer(t)
	sv := &saver{err: errors.New("disk full")}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	if m.state != stateReady {
		t.Fatalf("state = %d, want ready", m.state)
	}
	if m.chapter == nil || m.chapter.Chapter != 3 {
		t.Fatalf("chapter discarded: %+v", m.chapter)
	}
	if !strings.Contains(m.status, "could not save") {
		t.Fatalf("status = %q", m.status)
	}
	view := m.View()
	if !strings.Contains(view, "could not save") {
		t.Fatalf("status missing from view:\n%s", view)
	}
	_, cmd := m.Update(key("q"))
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q after save failure: %T", msg)
	}
}

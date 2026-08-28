package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
)

func TestSearchSubmitResultsOpenAtVerse(t *testing.T) {
	srv, log := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	saves := sv.n

	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewSearchQuery {
		t.Fatalf("view = %d, want search query", m.view)
	}
	view := m.View()
	if !strings.Contains(view, "search:") {
		t.Fatalf("query prompt missing:\n%s", view)
	}
	if !strings.Contains(view, "q quit") {
		t.Fatalf("reader keymap missing from footer:\n%s", view)
	}

	next, cmd = m.Update(key("love"))
	m = drain(t, asModel(t, next), cmd)
	if m.searchQuery != "love" {
		t.Fatalf("query = %q", m.searchQuery)
	}
	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewSearchResults {
		t.Fatalf("view = %d, want results", m.view)
	}
	joined := strings.Join(log.paths, ",")
	if !strings.Contains(joined, "/v1/search") {
		t.Fatalf("paths = %v, want /v1/search", log.paths)
	}
	foundSearchQ := false
	for _, q := range log.queries {
		if strings.Contains(q, "q=love") && strings.Contains(q, "translation=web") {
			foundSearchQ = true
			break
		}
	}
	if !foundSearchQ {
		t.Fatalf("search query = %v", log.queries)
	}
	if len(m.searchHits) == 0 {
		t.Fatal("no hits")
	}
	view = m.View()
	if !strings.Contains(view, "Genesis 22:2") {
		t.Fatalf("hit missing:\n%s", view)
	}

	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewReader {
		t.Fatalf("view = %d, want reader", m.view)
	}
	if m.chapter == nil || m.chapter.Book.ID != "GEN" || m.chapter.Chapter != 22 {
		t.Fatalf("opened chapter = %+v", m.chapter)
	}
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want verse 2 index 1", m.cursor)
	}
	if sv.n != saves+1 || sv.book != "GEN" || sv.chapter != 22 {
		t.Fatalf("resume after search = %+v", sv)
	}
}

func TestSearchEscapeOneLevel(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, _ := m.Update(key("j"))
	m = asModel(t, next)

	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("love"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewSearchResults {
		t.Fatalf("view = %d", m.view)
	}
	next, _ = m.Update(key("esc"))
	m = asModel(t, next)
	if m.view != viewSearchQuery {
		t.Fatalf("esc from results = %d, want query", m.view)
	}
	if m.searchQuery != "love" {
		t.Fatalf("query lost: %q", m.searchQuery)
	}
	next, _ = m.Update(key("esc"))
	m = asModel(t, next)
	if m.view != viewReader {
		t.Fatalf("esc from query = %d, want reader", m.view)
	}
	if m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 || m.cursor != 1 {
		t.Fatalf("reader mutated: %+v cursor=%d", m.chapter, m.cursor)
	}
}

func TestDelayedSearchDoesNotRestoreResultsAfterEscape(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("love"))
	m = drain(t, asModel(t, next), cmd)
	next, searchCmd := m.Update(key("enter"))
	m = asModel(t, next)
	if m.view != viewSearchResults {
		t.Fatalf("view = %d, want results", m.view)
	}
	if searchCmd == nil {
		t.Fatal("expected in-flight search")
	}
	next, _ = m.Update(key("esc"))
	m = asModel(t, next)
	if m.view != viewSearchQuery {
		t.Fatalf("esc from results = %d, want query", m.view)
	}
	next, _ = m.Update(searchCmd())
	m = asModel(t, next)
	if m.view != viewSearchQuery {
		t.Fatalf("delayed search restored results: view=%d", m.view)
	}
	next, _ = m.Update(searchMsg{
		seq: m.searchSeq,
		resp: &api.SearchResponse{
			Query: "love",
			Total: 1,
			Results: []api.SearchHit{{
				Book:      "GEN",
				BookName:  "Genesis",
				Chapter:   22,
				Verse:     2,
				Text:      "Take your son",
				Reference: "Genesis 22:2",
			}},
		},
	})
	m = asModel(t, next)
	if m.view != viewSearchQuery {
		t.Fatalf("current-seq search restored results: view=%d", m.view)
	}
}

func TestStaleSearchRejected(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("love"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("enter"))
	m = asModel(t, next)
	next, _ = m.Update(searchMsg{seq: 0, resp: nil, err: nil})
	m = asModel(t, next)
	m = drain(t, m, cmd)
	if len(m.searchHits) == 0 {
		t.Fatal("live search results missing")
	}
	if m.view != viewSearchResults {
		t.Fatalf("view = %d", m.view)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	srv, log := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	before := len(log.paths)
	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("enter"))
	m = asModel(t, next)
	if cmd != nil {
		t.Fatal("empty query must not search")
	}
	if m.view != viewSearchQuery {
		t.Fatalf("view = %d", m.view)
	}
	if !strings.Contains(m.status, "empty") {
		t.Fatalf("status = %q", m.status)
	}
	if len(log.paths) != before {
		t.Fatalf("extra requests: %v", log.paths[before:])
	}
}

func TestQClosesSearchQueryWithoutQuit(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewSearchQuery {
		t.Fatalf("view = %d, want search query", m.view)
	}
	view := m.View()
	if !strings.Contains(view, "q close") {
		t.Fatalf("footer missing q close:\n%s", view)
	}
	next, cmd = m.Update(key("q"))
	m = asModel(t, next)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatal("q on search query must not quit")
			}
		}
	}
	if m.view != viewReader {
		t.Fatalf("view = %d, want reader", m.view)
	}
}

func TestCtrlCQuitsFromSearch(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	next, cmd := m.Update(key("/"))
	m = drain(t, asModel(t, next), cmd)
	_, cmd = m.Update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c must quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("got %T", msg)
	}
}

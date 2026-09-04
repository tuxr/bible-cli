package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
)

func TestGroupBooksOTNTAP(t *testing.T) {
	books := []api.Book{
		{ID: "MAT", Name: "Matthew", Testament: "NT"},
		{ID: "GEN", Name: "Genesis", Testament: "OT"},
		{ID: "TOB", Name: "Tobit", Testament: "AP"},
		{ID: "EXO", Name: "Exodus", Testament: "OT"},
		{ID: "ZZZ", Name: "Other", Testament: "XX"},
	}
	got := groupBooks(books)
	want := []string{"GEN", "EXO", "MAT", "TOB", "ZZZ"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("index %d = %s, want %s", i, got[i].ID, want[i])
		}
	}
}

func TestBookFilterIDNameAlias(t *testing.T) {
	john := api.Book{ID: "JHN", Name: "John", Testament: "NT", Aliases: []string{"Jn", "Jhn"}}
	tests := []struct {
		q    string
		want bool
	}{
		{"jhn", true},
		{"JOHN", true},
		{"jn", true},
		{"oh", true},
		{"genesis", false},
	}
	for _, tt := range tests {
		if bookMatches(john, tt.q) != tt.want {
			t.Fatalf("bookMatches(%q)=%v, want %v", tt.q, !tt.want, tt.want)
		}
	}
}

func TestBookPickerFilterAndChapterSelect(t *testing.T) {
	srv, log := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1, SaveResume: sv.save}))
	if sv.n != 1 {
		t.Fatalf("initial saves = %d", sv.n)
	}
	next, cmd := m.Update(key("b"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewBooks {
		t.Fatalf("view = %d, want books", m.view)
	}
	if !strings.Contains(strings.Join(log.paths, ","), "/v1/books") {
		t.Fatalf("paths = %v, want /v1/books", log.paths)
	}
	view := m.View()
	if !strings.Contains(view, "NT") {
		t.Fatalf("expected NT group:\n%s", view)
	}
	if !strings.Contains(view, "esc back") || !strings.Contains(view, "q quit") {
		t.Fatalf("footer should keep overlay + reader keys:\n%s", view)
	}

	next, cmd = m.Update(key("jn"))
	m = drain(t, asModel(t, next), cmd)
	items := m.filteredBooks()
	if len(items) == 0 || items[0].ID != "JHN" {
		t.Fatalf("filter jn = %+v", items)
	}

	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewChapters {
		t.Fatalf("view = %d, want chapters", m.view)
	}
	if m.pickedBook.ID != "JHN" {
		t.Fatalf("picked = %+v", m.pickedBook)
	}

	next, _ = m.Update(key("down"))
	m = asModel(t, next)
	next, _ = m.Update(key("down"))
	m = asModel(t, next)
	if m.chapCursor != 2 {
		t.Fatalf("chapCursor = %d, want 2 (chapter 3)", m.chapCursor)
	}

	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewReader {
		t.Fatalf("view = %d, want reader", m.view)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 {
		t.Fatalf("chapter = %+v", m.chapter)
	}
	if sv.n != 2 || sv.book != "JHN" || sv.chapter != 3 {
		t.Fatalf("resume after book pick = %+v", sv)
	}
}

func TestBookPickerScrollsToSelection(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		first  string
		last   string
	}{
		{name: "all books", first: "MAT", last: "REV"},
		{name: "filtered", filter: "o", first: "JHN", last: "REV"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newTUIServer(t)
			m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
			next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
			m = asModel(t, next)
			next, cmd := m.Update(key("b"))
			m = drain(t, asModel(t, next), cmd)
			if tt.filter != "" {
				next, _ = m.Update(key(tt.filter))
				m = asModel(t, next)
			}
			items := m.filteredBooks()
			if len(items) == 0 || items[0].ID != tt.first || items[len(items)-1].ID != tt.last {
				t.Fatalf("filtered books = %+v", items)
			}
			row := func(b api.Book) string { return fmt.Sprintf(" %s  %s", b.ID, b.Name) }
			first, last := row(items[0]), row(items[len(items)-1])
			topPage := func(view string) {
				t.Helper()
				if !strings.Contains(view, first) || strings.Contains(view, last) {
					t.Fatalf("top page should show %s and hide %s:\n%s", tt.first, tt.last, view)
				}
				if tt.filter != "" && !strings.Contains(view, "filter: "+tt.filter) {
					t.Fatalf("filter echo hidden on top page:\n%s", view)
				}
			}
			press := func(k string) {
				t.Helper()
				next, _ := m.Update(key(k))
				m = asModel(t, next)
				if sel := row(items[m.bookCursor]); !strings.Contains(m.View(), sel) {
					t.Fatalf("%s: selected %q scrolled off screen:\n%s", k, sel, m.View())
				}
			}

			topPage(m.View())
			for m.bookCursor < len(items)-1 {
				press("down")
			}
			view := m.View()
			if !strings.Contains(view, last) || strings.Contains(view, first) {
				t.Fatalf("bottom page should show %s and hide %s:\n%s", tt.last, tt.first, view)
			}
			for m.bookCursor > 0 {
				press("up")
			}
			topPage(m.View())
		})
	}
}

func TestTranslationSwitchSuccess(t *testing.T) {
	srv, log := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	saves := sv.n
	oldID := m.translation
	next, cmd := m.Update(key("t"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewTranslations {
		t.Fatalf("view = %d", m.view)
	}
	if !strings.Contains(strings.Join(log.paths, ","), "/v1/translations") {
		t.Fatalf("paths = %v", log.paths)
	}
	if m.transCursor < 0 || m.transCursor >= len(m.translations) || !strings.EqualFold(m.translations[m.transCursor].ID, "web") {
		t.Fatalf("cursor not on current translation: %+v idx=%d", m.translations, m.transCursor)
	}
	for m.transCursor > 0 {
		next, _ = m.Update(key("up"))
		m = asModel(t, next)
	}
	if m.translations[m.transCursor].ID != "kjv" {
		t.Fatalf("expected kjv at top, got %s", m.translations[m.transCursor].ID)
	}
	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewReader {
		t.Fatalf("view = %d", m.view)
	}
	if !strings.EqualFold(m.translation, "kjv") {
		t.Fatalf("translation = %q, want kjv (was %s)", m.translation, oldID)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 {
		t.Fatalf("chapter moved: %+v", m.chapter)
	}
	if m.chapter.Translation.ID != "kjv" {
		t.Fatalf("chapter translation = %q", m.chapter.Translation.ID)
	}
	if sv.n != saves+1 {
		t.Fatalf("expected resume save on translation success, saver=%+v", sv)
	}
}

func TestTranslation404Rollback(t *testing.T) {
	srv, _ := newTUIServer(t)
	sv := &saver{}
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3, SaveResume: sv.save}))
	saves := sv.n
	next, cmd := m.Update(key("t"))
	m = drain(t, asModel(t, next), cmd)
	for m.transCursor < len(m.translations)-1 {
		next, _ = m.Update(key("down"))
		m = asModel(t, next)
	}
	if m.translations[m.transCursor].ID != "wlc" {
		t.Fatalf("expected wlc, got %s", m.translations[m.transCursor].ID)
	}
	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if !strings.EqualFold(m.translation, "web") {
		t.Fatalf("translation mutated to %q", m.translation)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 {
		t.Fatalf("chapter mutated: %+v", m.chapter)
	}
	if m.status == "" {
		t.Fatal("expected status on 404")
	}
	if sv.n != saves {
		t.Fatalf("resume saved on failed translation: %+v", sv)
	}
	view := m.View()
	if !strings.Contains(view, m.status) {
		t.Fatalf("status missing from view:\n%s", view)
	}
}

func TestEscapeDoesNotMutateReader(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, _ := m.Update(key("j"))
	m = asModel(t, next)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d", m.cursor)
	}

	next, cmd := m.Update(key("b"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("jn"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("enter"))
	m = drain(t, asModel(t, next), cmd)
	if m.view != viewChapters {
		t.Fatalf("view = %d", m.view)
	}
	next, _ = m.Update(key("esc"))
	m = asModel(t, next)
	if m.view != viewBooks {
		t.Fatalf("esc from chapters = %d, want books", m.view)
	}
	next, _ = m.Update(key("esc"))
	m = asModel(t, next)
	if m.view != viewReader {
		t.Fatalf("esc from books = %d, want reader", m.view)
	}
	if m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 || m.cursor != 1 {
		t.Fatalf("reader mutated: chapter=%+v cursor=%d", m.chapter, m.cursor)
	}
}

func TestQClosesOverlayWithoutQuit(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	next, cmd := m.Update(key("b"))
	m = drain(t, asModel(t, next), cmd)
	next, cmd = m.Update(key("q"))
	m = asModel(t, next)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatal("q on picker must not quit")
			}
		}
	}
	if m.view != viewReader {
		t.Fatalf("view = %d, want reader", m.view)
	}
}

func TestStaleBooksRejected(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "GEN", ResumeChapter: 1}))
	next, cmd := m.Update(key("b"))
	m = asModel(t, next)
	stale := booksMsg{seq: 0, books: []api.Book{{ID: "ZZZ", Name: "Nope", Testament: "OT"}}}
	next, _ = m.Update(stale)
	m = asModel(t, next)
	for _, b := range m.books {
		if b.ID == "ZZZ" {
			t.Fatal("stale books applied")
		}
	}
	m = drain(t, m, cmd)
	if len(m.books) == 0 {
		t.Fatal("expected live books after drain")
	}
}

func TestDelayedChapterDoesNotCloseBooksOverlay(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	cursor := m.cursor
	next, navCmd := m.Update(key("n"))
	m = asModel(t, next)
	next, booksCmd := m.Update(key("b"))
	m = drain(t, asModel(t, next), booksCmd)
	if m.view != viewBooks {
		t.Fatalf("view = %d, want books", m.view)
	}
	if navCmd == nil {
		t.Fatal("expected in-flight chapter fetch")
	}
	next, _ = m.Update(navCmd())
	m = asModel(t, next)
	if m.view != viewBooks {
		t.Fatalf("delayed chapter closed overlay: view=%d", m.view)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 || m.cursor != cursor {
		t.Fatalf("reader mutated: %+v cursor=%d", m.chapter, m.cursor)
	}
	next, _ = m.Update(chapterMsg{
		seq:     m.chapterSeq,
		chapter: mustChapter(t, "JHN", "John", 4, []api.Verse{{Verse: 1, Text: "Therefore when the Lord knew that the Pharisees had heard."}}, &api.NavRef{Book: "JHN", Chapter: 3}, &api.NavRef{Book: "JHN", Chapter: 5}),
		book:    "JHN",
		n:       4,
	})
	m = asModel(t, next)
	if m.view != viewBooks {
		t.Fatalf("current-seq chapter closed overlay: view=%d", m.view)
	}
	if m.chapter.Chapter != 3 {
		t.Fatalf("reader chapter = %d", m.chapter.Chapter)
	}
}

func TestDelayedChapterDoesNotCloseTranslationsOverlay(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	cursor := m.cursor
	next, navCmd := m.Update(key("n"))
	m = asModel(t, next)
	next, transCmd := m.Update(key("t"))
	m = drain(t, asModel(t, next), transCmd)
	if m.view != viewTranslations {
		t.Fatalf("view = %d, want translations", m.view)
	}
	if navCmd == nil {
		t.Fatal("expected in-flight chapter fetch")
	}
	next, _ = m.Update(navCmd())
	m = asModel(t, next)
	if m.view != viewTranslations {
		t.Fatalf("delayed chapter closed overlay: view=%d", m.view)
	}
	if m.chapter == nil || m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 3 || m.cursor != cursor {
		t.Fatalf("reader mutated: %+v cursor=%d", m.chapter, m.cursor)
	}
	next, _ = m.Update(chapterMsg{
		seq:     m.chapterSeq,
		chapter: mustChapter(t, "JHN", "John", 4, []api.Verse{{Verse: 1, Text: "Therefore when the Lord knew that the Pharisees had heard."}}, &api.NavRef{Book: "JHN", Chapter: 3}, &api.NavRef{Book: "JHN", Chapter: 5}),
		book:    "JHN",
		n:       4,
	})
	m = asModel(t, next)
	if m.view != viewTranslations {
		t.Fatalf("current-seq chapter closed overlay: view=%d", m.view)
	}
	if m.chapter.Chapter != 3 {
		t.Fatalf("reader chapter = %d", m.chapter.Chapter)
	}
}

func TestStaleChapterRejected(t *testing.T) {
	srv, _ := newTUIServer(t)
	m := load(t, clientOpts(t, srv, Options{ResumeBook: "JHN", ResumeChapter: 3}))
	next, cmd := m.Update(key("n"))
	m = asModel(t, next)
	stale := mustChapter(t, "GEN", "Genesis", 1, []api.Verse{{Verse: 1, Text: "stale"}}, nil, nil)
	next, _ = m.Update(chapterMsg{seq: 0, chapter: stale, book: "GEN", n: 1})
	m = asModel(t, next)
	if m.chapter != nil && m.chapter.Book.ID == "GEN" {
		t.Fatal("stale chapter applied")
	}
	m = drain(t, m, cmd)
	if m.chapter.Book.ID != "JHN" || m.chapter.Chapter != 4 {
		t.Fatalf("after n: %+v", m.chapter)
	}
}

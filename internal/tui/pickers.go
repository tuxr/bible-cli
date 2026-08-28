package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
)

type booksMsg struct {
	seq   int
	books []api.Book
	err   error
}

type translationsMsg struct {
	seq  int
	list []api.Translation
	err  error
}

func groupBooks(books []api.Book) []api.Book {
	var ot, nt, ap, other []api.Book
	for _, b := range books {
		switch strings.ToUpper(strings.TrimSpace(b.Testament)) {
		case "OT":
			ot = append(ot, b)
		case "NT":
			nt = append(nt, b)
		case "AP":
			ap = append(ap, b)
		default:
			other = append(other, b)
		}
	}
	out := make([]api.Book, 0, len(books))
	out = append(out, ot...)
	out = append(out, nt...)
	out = append(out, ap...)
	out = append(out, other...)
	return out
}

func bookMatches(b api.Book, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(b.ID), q) || strings.Contains(strings.ToLower(b.Name), q) {
		return true
	}
	for _, a := range b.Aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	return false
}

func (m Model) filteredBooks() []api.Book {
	grouped := groupBooks(m.books)
	out := make([]api.Book, 0, len(grouped))
	for _, b := range grouped {
		if bookMatches(b, m.bookFilter) {
			out = append(out, b)
		}
	}
	return out
}

func (m Model) openBooks() (Model, tea.Cmd) {
	m.help = false
	m.view = viewBooks
	m.bookFilter = ""
	m.bookCursor = 0
	m.booksSeq++
	m.status = ""
	seq := m.booksSeq
	client := m.client
	return m, func() tea.Msg {
		if client == nil {
			return booksMsg{seq: seq, err: fmt.Errorf("no client")}
		}
		books, err := client.Books(context.Background(), "")
		return booksMsg{seq: seq, books: books, err: err}
	}
}

func (m Model) openTranslations() (Model, tea.Cmd) {
	m.help = false
	m.view = viewTranslations
	m.transCursor = 0
	m.transSeq++
	m.status = ""
	seq := m.transSeq
	client := m.client
	return m, func() tea.Msg {
		if client == nil {
			return translationsMsg{seq: seq, err: fmt.Errorf("no client")}
		}
		list, err := client.Translations(context.Background())
		return translationsMsg{seq: seq, list: list, err: err}
	}
}

func (m Model) applyBooks(msg booksMsg) (Model, tea.Cmd) {
	if msg.seq != m.booksSeq {
		return m, nil
	}
	if m.view != viewBooks {
		return m, nil
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		return m, nil
	}
	m.books = msg.books
	m.bookCursor = 0
	return m, nil
}

func (m Model) applyTranslations(msg translationsMsg) (Model, tea.Cmd) {
	if msg.seq != m.transSeq {
		return m, nil
	}
	if m.view != viewTranslations {
		return m, nil
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		return m, nil
	}
	m.translations = msg.list
	m.transCursor = 0
	for i, tr := range m.translations {
		if strings.EqualFold(tr.ID, m.translation) {
			m.transCursor = i
			break
		}
	}
	return m, nil
}

func (m Model) handleBooksKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewReader
		return m, nil
	case "q":
		return m.closeOverlays(), nil
	case "enter":
		items := m.filteredBooks()
		if m.bookCursor < 0 || m.bookCursor >= len(items) {
			return m, nil
		}
		m.pickedBook = items[m.bookCursor]
		m.chapCursor = 0
		m.view = viewChapters
		return m, nil
	case "up":
		if m.bookCursor > 0 {
			m.bookCursor--
		}
		return m, nil
	case "down":
		if m.bookCursor < len(m.filteredBooks())-1 {
			m.bookCursor++
		}
		return m, nil
	case "backspace":
		if m.bookFilter != "" {
			r := []rune(m.bookFilter)
			m.bookFilter = string(r[:len(r)-1])
			m.bookCursor = 0
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.bookFilter += string(msg.Runes)
		m.bookCursor = 0
	}
	return m, nil
}

func (m Model) handleChaptersKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	maxCh := m.pickedBook.Chapters
	if maxCh < 1 {
		maxCh = 1
	}
	switch msg.String() {
	case "esc":
		m.view = viewBooks
		return m, nil
	case "q":
		return m.closeOverlays(), nil
	case "enter":
		ch := m.chapCursor + 1
		if ch < 1 {
			ch = 1
		}
		if ch > maxCh {
			ch = maxCh
		}
		return m.startChapter(m.pickedBook.ID, ch, 0, "", false)
	case "j", "down", "right", "l":
		if m.chapCursor < maxCh-1 {
			m.chapCursor++
		}
		return m, nil
	case "k", "up", "left", "h":
		if m.chapCursor > 0 {
			m.chapCursor--
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleTranslationsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewReader
		return m, nil
	case "q":
		return m.closeOverlays(), nil
	case "enter":
		if m.transCursor < 0 || m.transCursor >= len(m.translations) {
			return m, nil
		}
		tr := m.translations[m.transCursor]
		if m.chapter == nil {
			m.status = "no chapter loaded"
			m.view = viewReader
			return m, nil
		}
		if strings.EqualFold(tr.ID, m.translation) {
			m.view = viewReader
			return m, nil
		}
		return m.startChapter(m.chapter.Book.ID, m.chapter.Chapter, 0, tr.ID, true)
	case "j", "down":
		if m.transCursor < len(m.translations)-1 {
			m.transCursor++
		}
		return m, nil
	case "k", "up":
		if m.transCursor > 0 {
			m.transCursor--
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderBookPicker(width, height int) string {
	items := m.filteredBooks()
	var b strings.Builder
	if m.books == nil && m.status == "" {
		b.WriteString("loading books…")
	} else {
		if m.bookFilter != "" {
			b.WriteString("filter: " + m.bookFilter + "\n")
		}
		var last string
		for i, book := range items {
			tes := strings.ToUpper(strings.TrimSpace(book.Testament))
			if tes != last {
				last = tes
				b.WriteString(tes + "\n")
			}
			line := fmt.Sprintf(" %s  %s", book.ID, book.Name)
			if i == m.bookCursor {
				line = m.selectedStyle().Width(max(1, width)).Render(clip(line, width))
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
		if len(items) == 0 && m.books != nil {
			b.WriteString("no matches")
		}
	}
	return window(wrap(strings.TrimRight(b.String(), "\n"), width), height, 0)
}

func (m Model) renderChapterGrid(width, height int) string {
	name := m.pickedBook.Name
	if name == "" {
		name = m.pickedBook.ID
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s  %d chapters\n", name, m.pickedBook.Chapters))
	cols := max(1, width/4)
	n := m.pickedBook.Chapters
	for i := 0; i < n; i++ {
		if i > 0 && i%cols == 0 {
			b.WriteByte('\n')
		}
		cell := fmt.Sprintf("%3d", i+1)
		if i == m.chapCursor {
			cell = m.selectedStyle().Render(cell)
		}
		b.WriteString(cell)
		b.WriteByte(' ')
	}
	return window(wrap(b.String(), width), height, 0)
}

func (m Model) renderTranslationPicker(width, height int) string {
	var b strings.Builder
	if m.translations == nil && m.status == "" {
		b.WriteString("loading translations…")
	} else {
		for i, tr := range m.translations {
			mark := " "
			if strings.EqualFold(tr.ID, m.translation) {
				mark = "*"
			}
			line := fmt.Sprintf("%s %s  %s", mark, tr.ID, tr.Name)
			if i == m.transCursor {
				line = m.selectedStyle().Width(max(1, width)).Render(clip(line, width))
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return window(wrap(strings.TrimRight(b.String(), "\n"), width), height, 0)
}

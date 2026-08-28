package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/render"
)

type searchMsg struct {
	seq  int
	resp *api.SearchResponse
	err  error
}

func (m Model) openSearch() (Model, tea.Cmd) {
	m.help = false
	m.view = viewSearchQuery
	m.searchQuery = ""
	m.searchHits = nil
	m.searchCursor = 0
	m.searchTotal = 0
	m.status = ""
	return m, nil
}

func (m Model) closeOverlays() Model {
	m.view = viewReader
	m.bookFilter = ""
	return m
}

func (m Model) handleSearchQueryKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewReader
		return m, nil
	case "enter":
		return m.submitSearch()
	case "backspace":
		if m.searchQuery != "" {
			r := []rune(m.searchQuery)
			m.searchQuery = string(r[:len(r)-1])
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.searchQuery += string(msg.Runes)
	}
	return m, nil
}

func (m Model) submitSearch() (Model, tea.Cmd) {
	q := strings.TrimSpace(m.searchQuery)
	if q == "" {
		m.status = "empty query"
		return m, nil
	}
	m.searchSeq++
	m.status = ""
	m.searchHits = nil
	m.searchCursor = 0
	m.view = viewSearchResults
	seq := m.searchSeq
	client := m.client
	translation := m.translation
	return m, func() tea.Msg {
		if client == nil {
			return searchMsg{seq: seq, err: fmt.Errorf("no client")}
		}
		resp, err := client.Search(context.Background(), api.SearchQuery{
			Q:           q,
			Translation: translation,
		})
		return searchMsg{seq: seq, resp: resp, err: err}
	}
}

func (m Model) applySearch(msg searchMsg) (Model, tea.Cmd) {
	if msg.seq != m.searchSeq {
		return m, nil
	}
	if m.view != viewSearchResults && m.view != viewSearchQuery {
		return m, nil
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		m.view = viewSearchQuery
		return m, nil
	}
	if msg.resp == nil {
		m.status = "empty search"
		m.view = viewSearchQuery
		return m, nil
	}
	m.searchHits = msg.resp.Results
	m.searchTotal = msg.resp.Total
	m.searchCursor = 0
	m.view = viewSearchResults
	if len(m.searchHits) == 0 {
		m.status = "no results"
	}
	return m, nil
}

func (m Model) handleSearchResultsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewSearchQuery
		return m, nil
	case "q":
		return m.closeOverlays(), nil
	case "enter":
		if m.searchCursor < 0 || m.searchCursor >= len(m.searchHits) {
			return m, nil
		}
		hit := m.searchHits[m.searchCursor]
		if strings.TrimSpace(hit.Book) == "" || hit.Chapter < 1 {
			m.status = "incomplete search hit"
			return m, nil
		}
		return m.startChapter(hit.Book, hit.Chapter, hit.Verse, "", false)
	case "j", "down":
		if m.searchCursor < len(m.searchHits)-1 {
			m.searchCursor++
		}
		return m, nil
	case "k", "up":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderSearchQuery(width, height int) string {
	text := "search: " + m.searchQuery + "█"
	return window(wrap(text, width), height, 0)
}

func (m Model) renderSearchResults(width, height int) string {
	var b strings.Builder
	q := strings.TrimSpace(m.searchQuery)
	if q != "" {
		b.WriteString(fmt.Sprintf("search %q", q))
		if m.searchTotal > 0 {
			b.WriteString(fmt.Sprintf("  %d", m.searchTotal))
		}
		b.WriteByte('\n')
	}
	if m.searchHits == nil && m.status == "" {
		b.WriteString("searching…")
	} else if len(m.searchHits) == 0 {
		b.WriteString("no results")
	} else {
		opts := render.Options{
			Color:     m.color,
			RedLetter: m.redLetter,
			Palette:   m.pal,
		}
		for i, hit := range m.searchHits {
			line := searchHitLine(hit, opts)
			if i == m.searchCursor {
				line = m.selectedStyle().Width(max(1, width)).Render(wrap(line, width))
			} else {
				line = wrap(line, width)
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return window(wrap(strings.TrimRight(b.String(), "\n"), width), height, 0)
}

func searchHitLine(hit api.SearchHit, opts render.Options) string {
	ref := strings.TrimSpace(hit.Reference)
	if ref == "" {
		name := strings.TrimSpace(hit.BookName)
		if name == "" {
			name = strings.TrimSpace(hit.Book)
		}
		ref = fmt.Sprintf("%s %d:%d", name, hit.Chapter, hit.Verse)
	}
	// Search payloads often omit segments; unmarked text is the v1 fallback.
	body := strings.TrimSpace(hit.Text)
	if body == "" {
		v := api.Verse{Verse: hit.Verse, Text: hit.Text}
		line := render.VerseLine(v, opts)
		if i := strings.Index(line, "  "); i >= 0 {
			body = strings.TrimSpace(line[i+2:])
		}
	}
	return ref + "  " + body
}

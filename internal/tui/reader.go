package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tuxr/bible-cli/internal/render"
)

const footerKeys = " j/k verse  n/p chapter  ? help  q quit "

// Run starts the full-screen chapter reader.
func Run(opts Options) error {
	_, err := tea.NewProgram(New(opts), tea.WithAltScreen()).Run()
	return err
}

// View renders masthead, chapter (or overlay), status, and footer.
func (m Model) View() string {
	w := m.width
	if w < 1 {
		w = 1
	}
	h := m.height
	if h < 1 {
		h = 1
	}

	mast := m.renderMasthead(w)
	status := m.renderStatus(w)
	foot := m.renderFooter(w)
	used := lipgloss.Height(mast) + lipgloss.Height(status) + lipgloss.Height(foot)
	bodyH := h - used
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.renderBody(w, bodyH)
	return mast + "\n" + body + "\n" + status + "\n" + foot
}

func (m Model) renderMasthead(width int) string {
	left := " bible"
	book, ch, trans := m.location()
	mid := strings.TrimSpace(book)
	if ch > 0 {
		if mid != "" {
			mid = fmt.Sprintf("%s %d", mid, ch)
		} else {
			mid = fmt.Sprintf("%d", ch)
		}
	}
	if trans != "" {
		if mid != "" {
			mid += " · " + trans
		} else {
			mid = trans
		}
	}
	rule := m.fg(m.pal.Border).Render(repeat("─", width))
	title := m.fg(m.pal.Accent).Render(clip(left, width))
	ref := m.fg(m.pal.FgMuted).Render(clip(" "+mid, width))
	return title + "\n" + ref + "\n" + rule
}

func (m Model) location() (book string, chapter int, trans string) {
	if m.chapter != nil {
		book = m.chapter.Book.Name
		if book == "" {
			book = m.chapter.Book.ID
		}
		chapter = m.chapter.Chapter
		trans = strings.ToUpper(m.chapter.Translation.ID)
		return book, chapter, trans
	}
	trans = strings.ToUpper(m.translation)
	return book, chapter, trans
}

func (m Model) renderStatus(width int) string {
	text := m.status
	style := m.fg(m.pal.FgDim)
	if m.state == stateError && text == "" && m.err != nil {
		text = m.err.Error()
	}
	if m.state == stateError {
		style = m.fg(m.pal.Error)
	}
	return style.Render(clip(text, width))
}

func (m Model) renderFooter(width int) string {
	s := lipgloss.NewStyle()
	if m.color {
		s = s.Foreground(lipgloss.Color(m.pal.FooterFg)).Background(lipgloss.Color(m.pal.FooterBg))
	}
	return s.Width(width).MaxWidth(width).Render(clip(footerKeys, width))
}

func (m Model) renderBody(width, height int) string {
	var text string
	switch {
	case m.help:
		text = m.helpText()
	case m.state == stateLoading && m.chapter == nil:
		text = "loading…"
	case m.state == stateError && m.chapter == nil:
		if m.err != nil {
			text = m.err.Error()
		} else {
			text = m.status
		}
	default:
		text = m.verseBlock(width)
	}
	text = wrap(text, width)
	return window(text, height, m.cursorLine())
}

func (m Model) helpText() string {
	return strings.Join([]string{
		"keyboard",
		" j / ↓    next verse",
		" k / ↑    previous verse",
		" n / →    next chapter",
		" p / ←    previous chapter",
		" ?        toggle help",
		" q        quit",
	}, "\n")
}

func (m Model) verseBlock(width int) string {
	if m.chapter == nil {
		return ""
	}
	opts := render.Options{
		Color:     m.color,
		RedLetter: m.redLetter,
		Palette:   m.pal,
	}
	var b strings.Builder
	for i, v := range m.chapter.Verses {
		if i > 0 {
			b.WriteByte('\n')
		}
		line := render.VerseLine(v, opts)
		if i == m.cursor {
			line = m.selectedStyle().Width(max(1, width)).Render(wrap(line, width))
		} else {
			line = wrap(line, width)
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m Model) selectedStyle() lipgloss.Style {
	s := lipgloss.NewStyle()
	if m.color {
		s = s.Background(lipgloss.Color(m.pal.Accent)).Foreground(lipgloss.Color(m.pal.Bg))
	}
	return s
}

func (m Model) cursorLine() int {
	if m.chapter == nil || m.cursor <= 0 {
		return 0
	}
	opts := render.Options{Color: false, RedLetter: false, Palette: m.pal}
	lines := 0
	w := m.width
	if w < 1 {
		w = 1
	}
	for i, v := range m.chapter.Verses {
		if i >= m.cursor {
			break
		}
		block := wrap(render.VerseLine(v, opts), w)
		lines += lipgloss.Height(block)
	}
	return lines
}

func (m Model) fg(color string) lipgloss.Style {
	s := lipgloss.NewStyle()
	if m.color && color != "" {
		s = s.Foreground(lipgloss.Color(color))
	}
	return s
}

func wrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	if s == "" {
		return ""
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(s)
}

func window(text string, height, cursorLine int) string {
	if height < 1 {
		height = 1
	}
	rows := strings.Split(text, "\n")
	if len(rows) <= height {
		return text
	}
	start := cursorLine
	if start < 0 {
		start = 0
	}
	if start > len(rows)-height {
		start = len(rows) - height
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(rows) {
		end = len(rows)
	}
	return strings.Join(rows[start:end], "\n")
}

func clip(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func repeat(s string, n int) string {
	if n < 1 {
		return ""
	}
	return strings.Repeat(s, n)
}

package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/theme"
)

const (
	defaultBook    = "GEN"
	defaultChapter = 1
)

// book then chapter number (optional verse/range). Leading book numbers like "1 John" are allowed.
var hasChapterRE = regexp.MustCompile(`(?i)^(?:\d+\s+)?[A-Za-z]+(?:\s+[A-Za-z]+)*\s+\d+`)

type state int

const (
	stateLoading state = iota
	stateReady
	stateError
)

// Client is the scripture API surface the reader needs.
type Client interface {
	GetChapter(ctx context.Context, book string, chapter int, translation string) (*api.ChapterResponse, error)
	GetVerses(ctx context.Context, ref, translation string) (*api.VerseResponse, error)
	Books(ctx context.Context, testament string) ([]api.Book, error)
}

// ResumeWriter persists the last successfully loaded chapter.
type ResumeWriter func(book string, chapter int) error

// Options configure a new reader model.
type Options struct {
	Client        Client
	SaveResume    ResumeWriter
	Translation   string
	Ref           string
	ResumeBook    string
	ResumeChapter int
	Palette       theme.Palette
	Color         bool
	RedLetter     bool
}

// Model is the Bubble Tea chapter reader. Network I/O happens only in commands.
type Model struct {
	client      Client
	saveResume  ResumeWriter
	translation string
	ref         string
	resumeBook  string
	resumeCh    int
	pal         theme.Palette
	color       bool
	redLetter   bool

	state   state
	err     error
	status  string
	help    bool
	cursor  int
	width   int
	height  int
	chapter *api.ChapterResponse
}

type resolvedMsg struct {
	book    string
	chapter int
	err     error
}

type chapterMsg struct {
	chapter *api.ChapterResponse
	err     error
}

// New constructs a loading-state reader.
func New(opts Options) Model {
	t := strings.TrimSpace(opts.Translation)
	if t == "" {
		t = "web"
	}
	pal := opts.Palette
	if pal.Fg == "" && pal.Name == "" {
		pal = theme.Dark
	}
	return Model{
		client:      opts.Client,
		saveResume:  opts.SaveResume,
		translation: t,
		ref:         strings.TrimSpace(opts.Ref),
		resumeBook:  strings.TrimSpace(opts.ResumeBook),
		resumeCh:    opts.ResumeChapter,
		pal:         pal,
		color:       opts.Color,
		redLetter:   opts.RedLetter,
		state:       stateLoading,
		width:       80,
		height:      24,
	}
}

// Init starts ref resolution or the first chapter fetch.
func (m Model) Init() tea.Cmd {
	if m.ref != "" {
		return m.resolveRefCmd(m.ref)
	}
	if m.resumeBook != "" && m.resumeCh > 0 {
		return m.fetchChapterCmd(m.resumeBook, m.resumeCh)
	}
	return m.fetchChapterCmd(defaultBook, defaultChapter)
}

// Update handles window, fetch, and key messages. It never calls the network.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case resolvedMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		return m, m.fetchChapterCmd(msg.book, msg.chapter)
	case chapterMsg:
		return m.applyChapter(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) applyChapter(msg chapterMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		if m.chapter == nil {
			m.state = stateError
			m.err = msg.err
		}
		m.status = msg.err.Error()
		return m, nil
	}
	if msg.chapter == nil {
		if m.chapter == nil {
			m.state = stateError
			m.status = "empty chapter"
		} else {
			m.status = "empty chapter"
		}
		return m, nil
	}
	m.chapter = msg.chapter
	m.state = stateReady
	m.err = nil
	m.cursor = 0
	m.status = ""
	if m.saveResume != nil {
		if err := m.saveResume(msg.chapter.Book.ID, msg.chapter.Chapter); err != nil {
			m.status = "could not save position"
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.help = !m.help
		return m, nil
	case "j", "down":
		if m.chapter != nil && m.cursor < len(m.chapter.Verses)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "n", "right":
		return m.navigate(true)
	case "p", "left":
		return m.navigate(false)
	}
	return m, nil
}

func (m Model) navigate(next bool) (Model, tea.Cmd) {
	if m.chapter == nil {
		return m, nil
	}
	var nav *api.NavRef
	if next {
		nav = m.chapter.Navigation.Next
		if nav == nil {
			m.status = "end of canon"
			return m, nil
		}
	} else {
		nav = m.chapter.Navigation.Previous
		if nav == nil {
			m.status = "start of canon"
			return m, nil
		}
	}
	m.status = ""
	return m, m.fetchChapterCmd(nav.Book, nav.Chapter)
}

func (m Model) fetchChapterCmd(book string, chapter int) tea.Cmd {
	client := m.client
	translation := m.translation
	return func() tea.Msg {
		if client == nil {
			return chapterMsg{err: fmt.Errorf("no client")}
		}
		ch, err := client.GetChapter(context.Background(), book, chapter, translation)
		return chapterMsg{chapter: ch, err: err}
	}
}

func (m Model) resolveRefCmd(ref string) tea.Cmd {
	client := m.client
	translation := m.translation
	return func() tea.Msg {
		if client == nil {
			return resolvedMsg{err: fmt.Errorf("no client")}
		}
		ctx := context.Background()
		if hasChapterRE.MatchString(ref) {
			resp, err := client.GetVerses(ctx, ref, translation)
			if err != nil {
				return resolvedMsg{err: err}
			}
			if resp == nil || len(resp.Verses) == 0 {
				return resolvedMsg{err: fmt.Errorf("no verses for %s", ref)}
			}
			v := resp.Verses[0]
			if strings.TrimSpace(v.Book) == "" || v.Chapter < 1 {
				return resolvedMsg{err: fmt.Errorf("incomplete verse for %s", ref)}
			}
			return resolvedMsg{book: v.Book, chapter: v.Chapter}
		}
		books, err := client.Books(ctx, "")
		if err != nil {
			return resolvedMsg{err: err}
		}
		b, ok := matchBook(books, ref)
		if !ok {
			return resolvedMsg{err: fmt.Errorf("unknown book %q", ref)}
		}
		return resolvedMsg{book: b.ID, chapter: 1}
	}
}

func matchBook(books []api.Book, ref string) (api.Book, bool) {
	want := strings.TrimSpace(ref)
	for _, b := range books {
		if strings.EqualFold(b.ID, want) || strings.EqualFold(b.Name, want) {
			return b, true
		}
		for _, a := range b.Aliases {
			if strings.EqualFold(a, want) {
				return b, true
			}
		}
	}
	return api.Book{}, false
}

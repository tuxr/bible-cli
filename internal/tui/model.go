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

type viewMode int

const (
	viewReader viewMode = iota
	viewBooks
	viewChapters
	viewTranslations
	viewSearchQuery
	viewSearchResults
)

// Client is the scripture API surface the reader needs.
type Client interface {
	GetChapter(ctx context.Context, book string, chapter int, translation string) (*api.ChapterResponse, error)
	GetVerses(ctx context.Context, ref, translation string) (*api.VerseResponse, error)
	Books(ctx context.Context, testament string) ([]api.Book, error)
	Translations(ctx context.Context) ([]api.Translation, error)
	Search(ctx context.Context, q api.SearchQuery) (*api.SearchResponse, error)
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

	view       viewMode
	chapterSeq int
	booksSeq   int
	transSeq   int
	searchSeq  int

	books      []api.Book
	bookFilter string
	bookCursor int
	pickedBook api.Book
	chapCursor int

	translations []api.Translation
	transCursor  int

	searchQuery  string
	searchHits   []api.SearchHit
	searchCursor int
	searchTotal  int
}

type resolvedMsg struct {
	book    string
	chapter int
	err     error
}

type chapterMsg struct {
	seq               int
	chapter           *api.ChapterResponse
	book              string
	n                 int
	verse             int
	translation       string
	commitTranslation bool
	err               error
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
		return m.chapterCmd(m.resumeBook, m.resumeCh, 0, m.translation, false)
	}
	return m.chapterCmd(defaultBook, defaultChapter, 0, m.translation, false)
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
		return m, m.chapterCmd(msg.book, msg.chapter, 0, m.translation, false)
	case chapterMsg:
		return m.applyChapter(msg)
	case booksMsg:
		return m.applyBooks(msg)
	case translationsMsg:
		return m.applyTranslations(msg)
	case searchMsg:
		return m.applySearch(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) applyChapter(msg chapterMsg) (Model, tea.Cmd) {
	if msg.seq != m.chapterSeq {
		return m, nil
	}
	if m.view != viewReader {
		return m, nil
	}
	failed := msg.err != nil || msg.chapter == nil
	if failed {
		if msg.commitTranslation {
			if msg.err != nil {
				m.status = msg.err.Error()
			} else {
				m.status = "empty chapter"
			}
			m.view = viewReader
			return m, nil
		}
		if m.chapter == nil && m.ref == "" && !isDefaultChapter(msg.book, msg.n) {
			return m, m.chapterCmd(defaultBook, defaultChapter, 0, m.translation, false)
		}
		if msg.err != nil {
			if m.chapter == nil {
				m.state = stateError
				m.err = msg.err
			}
			m.status = msg.err.Error()
			m.view = viewReader
			return m, nil
		}
		if m.chapter == nil {
			m.state = stateError
			m.status = "empty chapter"
		} else {
			m.status = "empty chapter"
		}
		m.view = viewReader
		return m, nil
	}
	if msg.commitTranslation {
		if id := strings.TrimSpace(msg.chapter.Translation.ID); id != "" {
			m.translation = id
		} else if msg.translation != "" {
			m.translation = msg.translation
		}
	}
	m.chapter = msg.chapter
	m.state = stateReady
	m.err = nil
	m.cursor = cursorForVerse(msg.chapter.Verses, msg.verse)
	m.status = ""
	m.view = viewReader
	m.help = false
	if m.saveResume != nil {
		if err := m.saveResume(msg.chapter.Book.ID, msg.chapter.Chapter); err != nil {
			m.status = "could not save position"
		}
	}
	return m, nil
}

func cursorForVerse(verses []api.Verse, verse int) int {
	if verse < 1 {
		return 0
	}
	for i, v := range verses {
		if v.Verse == verse {
			return i
		}
	}
	return 0
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.view {
	case viewBooks:
		return m.handleBooksKey(msg)
	case viewChapters:
		return m.handleChaptersKey(msg)
	case viewTranslations:
		return m.handleTranslationsKey(msg)
	case viewSearchQuery:
		return m.handleSearchQueryKey(msg)
	case viewSearchResults:
		return m.handleSearchResultsKey(msg)
	}
	return m.handleReaderKey(msg)
}

func (m Model) handleReaderKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.help = !m.help
		return m, nil
	case "b":
		return m.openBooks()
	case "t":
		return m.openTranslations()
	case "/":
		return m.openSearch()
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
	return m.startChapter(nav.Book, nav.Chapter, 0, "", false)
}

func (m Model) startChapter(book string, chapter, verse int, translation string, commitTranslation bool) (Model, tea.Cmd) {
	m.chapterSeq++
	m.view = viewReader
	m.help = false
	if m.chapter == nil {
		m.state = stateLoading
	} else {
		m.status = "loading…"
	}
	return m, m.chapterCmd(book, chapter, verse, translation, commitTranslation)
}

func (m Model) chapterCmd(book string, chapter, verse int, translation string, commitTranslation bool) tea.Cmd {
	client := m.client
	if translation == "" {
		translation = m.translation
	}
	seq := m.chapterSeq
	return func() tea.Msg {
		if client == nil {
			return chapterMsg{
				seq: seq, book: book, n: chapter, verse: verse,
				translation: translation, commitTranslation: commitTranslation,
				err: fmt.Errorf("no client"),
			}
		}
		ch, err := client.GetChapter(context.Background(), book, chapter, translation)
		return chapterMsg{
			seq: seq, chapter: ch, book: book, n: chapter, verse: verse,
			translation: translation, commitTranslation: commitTranslation,
			err: err,
		}
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

func isDefaultChapter(book string, n int) bool {
	return strings.EqualFold(strings.TrimSpace(book), defaultBook) && n == defaultChapter
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

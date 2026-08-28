package api

// Translation is a bible-api translation metadata record.
type Translation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Language    string `json:"language,omitempty"`
	License     string `json:"license,omitempty"`
	Description string `json:"description,omitempty"`
}

// Segment is a speaker-tagged slice of verse text.
type Segment struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker,omitempty"`
}

// Verse is a single verse, optionally with speaker segments.
type Verse struct {
	Book     string    `json:"book,omitempty"`
	BookName string    `json:"book_name,omitempty"`
	Chapter  int       `json:"chapter,omitempty"`
	Verse    int       `json:"verse"`
	Text     string    `json:"text"`
	Segments []Segment `json:"segments,omitempty"`
}

// VerseResponse is GET /v1/verses/:ref.
type VerseResponse struct {
	Reference   string      `json:"reference"`
	Translation Translation `json:"translation"`
	Verses      []Verse     `json:"verses"`
}

// ChapterBook identifies the book a chapter belongs to.
type ChapterBook struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Testament string `json:"testament"`
}

// NavRef is a previous/next chapter pointer.
type NavRef struct {
	Book      string `json:"book"`
	Testament string `json:"testament"`
	Chapter   int    `json:"chapter"`
}

// Navigation is previous/next chapter links on a chapter payload.
type Navigation struct {
	Previous *NavRef `json:"previous"`
	Next     *NavRef `json:"next"`
}

// ChapterResponse is GET /v1/chapters/:book/:n.
type ChapterResponse struct {
	Book        ChapterBook `json:"book"`
	Chapter     int         `json:"chapter"`
	Translation Translation `json:"translation"`
	VerseCount  int         `json:"verse_count"`
	Verses      []Verse     `json:"verses"`
	Navigation  Navigation  `json:"navigation"`
}

// SearchHit is one search result.
type SearchHit struct {
	Book      string `json:"book"`
	BookName  string `json:"book_name"`
	Text      string `json:"text"`
	Reference string `json:"reference"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
}

// SearchResponse is GET /v1/search.
type SearchResponse struct {
	Query       string      `json:"query"`
	Translation string      `json:"translation"`
	Total       int         `json:"total"`
	Results     []SearchHit `json:"results"`
}

// Book is GET /v1/books item.
type Book struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Testament string   `json:"testament"`
	Chapters  int      `json:"chapters"`
	Aliases   []string `json:"aliases"`
}

// APIError is a classified bible-api failure.
// Kind is one of "not-found", "system", or "rate-limit".
type APIError struct {
	Status int    `json:"status,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Msg    string `json:"error"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

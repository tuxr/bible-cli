package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://bible-api.dws-cloud.com"
	apiPrefix      = "/v1"
	defaultVersion = "dev"
	repoURL        = "https://github.com/tuxr/bible-cli"
	httpTimeout    = 10 * time.Second
)

// Version is the bible-cli version used in User-Agent. Wired from main.
var Version = defaultVersion

// SearchQuery is GET /v1/search. Empty optional fields and Limit <= 0 are omitted.
type SearchQuery struct {
	Q           string
	Translation string
	Book        string
	Testament   string
	Limit       int
}

// Client talks to bible-api.
type Client struct {
	base string
	http *http.Client
}

// New returns a client for base (trailing slash stripped).
// An empty base uses https://bible-api.dws-cloud.com.
func New(base string) *Client {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		base: base,
		http: &http.Client{Timeout: httpTimeout},
	}
}

// GetVerses is GET /v1/verses/:ref.
func (c *Client) GetVerses(ctx context.Context, ref, translation string) (*VerseResponse, error) {
	var out VerseResponse
	q := scriptureQuery(translation)
	path := apiPrefix + "/verses/" + url.PathEscape(ref)
	if err := c.doJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetChapter is GET /v1/chapters/:book/:n.
func (c *Client) GetChapter(ctx context.Context, book string, chapter int, translation string) (*ChapterResponse, error) {
	var out ChapterResponse
	q := scriptureQuery(translation)
	path := apiPrefix + "/chapters/" + url.PathEscape(book) + "/" + strconv.Itoa(chapter)
	if err := c.doJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Search is GET /v1/search.
func (c *Client) Search(ctx context.Context, q SearchQuery) (*SearchResponse, error) {
	var out SearchResponse
	vals := url.Values{}
	vals.Set("q", q.Q)
	if q.Translation != "" {
		vals.Set("translation", q.Translation)
	}
	if q.Book != "" {
		vals.Set("book", q.Book)
	}
	if q.Testament != "" {
		vals.Set("testament", q.Testament)
	}
	if q.Limit > 0 {
		vals.Set("limit", strconv.Itoa(q.Limit))
	}
	if err := c.doJSON(ctx, apiPrefix+"/search", vals, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Books is GET /v1/books.
func (c *Client) Books(ctx context.Context, testament string) ([]Book, error) {
	var out []Book
	vals := url.Values{}
	if testament != "" {
		vals.Set("testament", testament)
	}
	if err := c.doJSON(ctx, apiPrefix+"/books", vals, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Translations is GET /v1/translations.
func (c *Client) Translations(ctx context.Context) ([]Translation, error) {
	var out []Translation
	if err := c.doJSON(ctx, apiPrefix+"/translations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Random is GET /v1/random.
func (c *Client) Random(ctx context.Context, translation, book, testament string) (*VerseResponse, error) {
	var out VerseResponse
	vals := scriptureQuery(translation)
	if book != "" {
		vals.Set("book", book)
	}
	if testament != "" {
		vals.Set("testament", testament)
	}
	if err := c.doJSON(ctx, apiPrefix+"/random", vals, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func scriptureQuery(translation string) url.Values {
	q := url.Values{}
	q.Set("segments", "1")
	if translation != "" {
		q.Set("translation", translation)
	}
	return q
}

func (c *Client) doJSON(ctx context.Context, path string, q url.Values, dest any) error {
	u := c.base + path
	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
	}

	resp, err := c.get(ctx, u)
	if err != nil {
		return systemErr(err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		wait := retryAfter(resp)
		_ = resp.Body.Close()
		if err := sleep(ctx, wait); err != nil {
			return systemErr(err)
		}
		resp, err = c.get(ctx, u)
		if err != nil {
			return systemErr(err)
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return systemErr(err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(body, dest); err != nil {
			return systemErr(err)
		}
		return nil
	}

	kind := "system"
	switch resp.StatusCode {
	case http.StatusBadRequest, http.StatusNotFound:
		kind = "not-found"
	case http.StatusTooManyRequests:
		kind = "rate-limit"
	}

	ae := &APIError{
		Status: resp.StatusCode,
		Kind:   kind,
		Msg:    fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
	var parsed APIError
	if json.Unmarshal(body, &parsed) == nil && parsed.Msg != "" {
		ae.Msg = parsed.Msg
	}
	return ae
}

func (c *Client) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", "application/json")
	return c.http.Do(req)
}

func userAgent() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = defaultVersion
	}
	return "bible-cli/" + v + " (+" + repoURL + ")"
}

func retryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func systemErr(err error) *APIError {
	return &APIError{Kind: "system", Msg: err.Error()}
}

package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetVersesJohn316Web(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	got, err := c.GetVerses(context.Background(), "John 3:16", "web")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "segments=1") {
		t.Fatalf("url %q missing segments=1", gotURL)
	}
	if !strings.Contains(gotURL, "translation=web") {
		t.Fatalf("url %q missing translation=web", gotURL)
	}
	if len(got.Verses) == 0 || len(got.Verses[0].Segments) == 0 {
		t.Fatal("expected verse segments")
	}
	if got.Verses[0].Segments[0].Speaker != "jesus" {
		t.Fatalf("speaker = %q, want jesus", got.Verses[0].Segments[0].Speaker)
	}
}

func TestGetVersesNotFound404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(readFixture(t, "error_404.json"))
	}))
	t.Cleanup(srv.Close)

	_, err := New(srv.URL).GetVerses(context.Background(), "Nope 1:1", "web")
	assertAPIKind(t, err, "not-found")
}

func TestGetVersesNotFound400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(readFixture(t, "error_400.json"))
	}))
	t.Cleanup(srv.Close)

	_, err := New(srv.URL).GetVerses(context.Background(), "John 3:0", "web")
	assertAPIKind(t, err, "not-found")
}

func TestGetVersesRetriesOnceOn429(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":"Too many requests"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	got, err := New(srv.URL).GetVerses(context.Background(), "John 3:16", "web")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("requests = %d, want 2", n)
	}
	if got.Reference != "John 3:16" {
		t.Fatalf("reference = %q", got.Reference)
	}
}

func TestUserAgent(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.UserAgent()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	if _, err := New(srv.URL).GetVerses(context.Background(), "John 3:16", "web"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ua, "bible-cli/") {
		t.Fatalf("User-Agent = %q, want prefix bible-cli/", ua)
	}
}

func TestGetChapterJohn3(t *testing.T) {
	var path, rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "chapter_john_3_segments.json"))
	}))
	t.Cleanup(srv.Close)

	got, err := New(srv.URL).GetChapter(context.Background(), "John", 3, "web")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/v1/chapters/John/3" {
		t.Fatalf("path = %q, want /v1/chapters/John/3", path)
	}
	if !strings.Contains(rawQuery, "segments=1") {
		t.Fatalf("query = %q missing segments=1", rawQuery)
	}
	if got.Chapter != 3 {
		t.Fatalf("chapter = %d", got.Chapter)
	}
}

func assertAPIKind(t *testing.T, err error, kind string) {
	t.Helper()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Kind != kind {
		t.Fatalf("Kind = %q, want %q", apiErr.Kind, kind)
	}
}

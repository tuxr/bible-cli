package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tuxr/bible-cli/internal/api"
)

func TestCatalogExecute(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		args      []string
		tty       bool
		wantCode  int
		wantPath  string
		queryHas  []string
		stderrHas string
		textHas   []string
		noAPI     bool
		checkJSON func(*testing.T, string)
	}{
		{
			name:     "search love piped json",
			fixture:  "search_love.json",
			args:     []string{"search", "love"},
			wantCode: 0,
			wantPath: "/v1/search",
			queryHas: []string{"q=love", "translation=web"},
			checkJSON: func(t *testing.T, stdout string) {
				t.Helper()
				var resp api.SearchResponse
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Fatalf("json: %v\n%s", err, stdout)
				}
				if resp.Query != "love" {
					t.Fatalf("query = %q", resp.Query)
				}
				if len(resp.Results) != 2 {
					t.Fatalf("len(results) = %d", len(resp.Results))
				}
				if resp.Results[0].Reference != "Genesis 22:2" {
					t.Fatalf("first reference = %q", resp.Results[0].Reference)
				}
			},
		},
		{
			name:     "search love tty numbered hits",
			fixture:  "search_love.json",
			args:     []string{"search", "love"},
			tty:      true,
			wantCode: 0,
			textHas:  []string{"1  Genesis 22:2  He said", "2  Genesis 27:4"},
		},
		{
			name:     "search --json always json on tty",
			fixture:  "search_love.json",
			args:     []string{"search", "love", "--json"},
			tty:      true,
			wantCode: 0,
			checkJSON: func(t *testing.T, stdout string) {
				t.Helper()
				var resp api.SearchResponse
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Fatalf("json: %v\n%s", err, stdout)
				}
				if resp.Query != "love" {
					t.Fatalf("query = %q", resp.Query)
				}
			},
		},
		{
			name:      "search empty query",
			args:      []string{"search"},
			noAPI:     true,
			wantCode:  1,
			stderrHas: "pass a search query",
		},
		{
			name:     "translations piped json web first",
			fixture:  "translations.json",
			args:     []string{"translations"},
			wantCode: 0,
			wantPath: "/v1/translations",
			checkJSON: func(t *testing.T, stdout string) {
				t.Helper()
				var list []api.Translation
				if err := json.Unmarshal([]byte(stdout), &list); err != nil {
					t.Fatalf("json: %v\n%s", err, stdout)
				}
				if len(list) == 0 {
					t.Fatal("empty translations")
				}
				if list[0].ID != "web" {
					t.Fatalf("first id = %q, want web", list[0].ID)
				}
			},
		},
		{
			name:     "translations tty web first",
			fixture:  "translations.json",
			args:     []string{"translations"},
			tty:      true,
			wantCode: 0,
			textHas:  []string{"web  World English Bible"},
		},
		{
			name:     "books NT piped json",
			fixture:  "books_nt.json",
			args:     []string{"books", "--testament", "NT"},
			wantCode: 0,
			wantPath: "/v1/books",
			queryHas: []string{"testament=NT"},
			checkJSON: func(t *testing.T, stdout string) {
				t.Helper()
				var list []api.Book
				if err := json.Unmarshal([]byte(stdout), &list); err != nil {
					t.Fatalf("json: %v\n%s", err, stdout)
				}
				if len(list) != 27 {
					t.Fatalf("len(books) = %d, want 27", len(list))
				}
				if list[0].ID != "MAT" || list[0].Testament != "NT" {
					t.Fatalf("first book = %+v", list[0])
				}
			},
		},
		{
			name:     "books NT tty",
			fixture:  "books_nt.json",
			args:     []string{"books", "--testament", "nt"},
			tty:      true,
			wantCode: 0,
			wantPath: "/v1/books",
			queryHas: []string{"testament=NT"},
			textHas:  []string{"MAT  Matthew  28"},
		},
		{
			name:      "books invalid testament",
			args:      []string{"books", "--testament", "XX"},
			noAPI:     true,
			wantCode:  1,
			stderrHas: "invalid testament",
		},
		{
			name:     "random --book PSA",
			fixture:  "verse_john_3_16_segments.json",
			args:     []string{"random", "--book", "PSA"},
			wantCode: 0,
			wantPath: "/v1/random",
			queryHas: []string{"book=PSA", "translation=web", "segments=1"},
			textHas:  []string{"For God so loved the world"},
		},
		{
			name:     "random --testament NT",
			fixture:  "verse_john_3_16_segments.json",
			args:     []string{"random", "--testament", "NT"},
			wantCode: 0,
			wantPath: "/v1/random",
			queryHas: []string{"testament=NT", "translation=web"},
			textHas:  []string{"For God so loved the world"},
		},
		{
			name:     "random --json",
			fixture:  "verse_john_3_16_segments.json",
			args:     []string{"random", "--json"},
			wantCode: 0,
			checkJSON: func(t *testing.T, stdout string) {
				t.Helper()
				var resp api.VerseResponse
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Fatalf("json: %v\n%s", err, stdout)
				}
				if resp.Reference != "John 3:16" {
					t.Fatalf("reference = %q", resp.Reference)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origTTY := writerIsTTY
			writerIsTTY = func(io.Writer) bool { return tc.tty }
			t.Cleanup(func() { writerIsTTY = origTTY })

			var path, rawQuery string
			called := false
			var srv *httptest.Server
			if tc.noAPI {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
				}))
			} else {
				body := readFixture(t, tc.fixture)
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					path = r.URL.Path
					rawQuery = r.URL.RawQuery
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(body)
				}))
			}
			t.Cleanup(srv.Close)

			stdout, stderr, code := runCLI(t, srv.URL, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d, stderr = %q stdout = %q", code, tc.wantCode, stderr, stdout)
			}
			if tc.noAPI && called {
				t.Fatal("must not call API")
			}
			if tc.wantPath != "" && path != tc.wantPath {
				t.Fatalf("path = %q, want %q", path, tc.wantPath)
			}
			for _, q := range tc.queryHas {
				if !strings.Contains(rawQuery, q) {
					t.Fatalf("query = %q, want %q", rawQuery, q)
				}
			}
			if tc.stderrHas != "" && !strings.Contains(stderr, tc.stderrHas) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.stderrHas)
			}
			for _, s := range tc.textHas {
				if !strings.Contains(stdout, s) {
					t.Fatalf("stdout missing %q:\n%s", s, stdout)
				}
			}
			if tc.checkJSON != nil {
				tc.checkJSON(t, stdout)
			}
		})
	}
}

func TestPreferWebFirst(t *testing.T) {
	in := []api.Translation{
		{ID: "kjv", Name: "King James Version"},
		{ID: "web", Name: "World English Bible"},
		{ID: "wlc", Name: "Westminster Leningrad Codex"},
	}
	got := preferWebFirst(in)
	if got[0].ID != "web" {
		t.Fatalf("first = %q", got[0].ID)
	}
	if got[1].ID != "kjv" || got[2].ID != "wlc" {
		t.Fatalf("rest = %+v", got)
	}
	if in[0].ID != "kjv" {
		t.Fatal("must not mutate input")
	}
}

func TestSearchUsesConfigTranslation(t *testing.T) {
	writeXDGConfig(t, "translation = \"kjv\"\n")
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "search_love.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "search", "love")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(gotQuery, "translation=kjv") {
		t.Fatalf("query = %q, want translation=kjv from config", gotQuery)
	}
}

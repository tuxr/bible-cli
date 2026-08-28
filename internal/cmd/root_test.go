package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tuxr/bible-cli/internal/api"
)

func isolateCmdEnv(t *testing.T) string {
	t.Helper()
	if os.Getenv("BIBLE_CLI_TEST_XDG") == "1" {
		return os.Getenv("XDG_CONFIG_HOME")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("BIBLE_TRANSLATION", "")
	t.Setenv("BIBLE_THEME", "")
	t.Setenv("BIBLE_API_URL", "")
	t.Setenv("BIBLE_RED_LETTER", "")
	t.Setenv("BIBLE_CLI_TEST_XDG", "1")
	return dir
}

func writeXDGConfig(t *testing.T, body string) {
	t.Helper()
	dir := isolateCmdEnv(t)
	cfgDir := filepath.Join(dir, "bible")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newFixtureServer(t *testing.T, status int, fixture string) *httptest.Server {
	t.Helper()
	body := readFixture(t, fixture)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runCLI(t *testing.T, apiURL string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	all := append([]string{"--api-url", apiURL}, args...)
	code = ExecuteWith(all, &out, &errb)
	return out.String(), errb.String(), code
}

func TestLookupJohn316PrintsWEBText(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "For God so loved the world") {
		t.Fatalf("stdout missing WEB text:\n%s", stdout)
	}
	if strings.Contains(stdout, "\x1b") {
		t.Fatalf("unexpected ANSI in captured stdout: %q", stdout)
	}
}

func TestLookupUnknownBookExit2(t *testing.T) {
	srv := newFixtureServer(t, http.StatusNotFound, "error_404.json")
	_, stderr, code := runCLI(t, srv.URL, "Nope", "1:1")
	if code != 2 {
		t.Fatalf("code = %d, want 2, stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "Not found") {
		t.Fatalf("stderr = %q, want Not found", stderr)
	}
}

func TestLookupJSONIncludesSegments(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "John", "3:16", "--json")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	var resp api.VerseResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(resp.Verses) == 0 || len(resp.Verses[0].Segments) == 0 {
		t.Fatalf("expected segments, got %+v", resp)
	}
	if resp.Verses[0].Segments[0].Speaker != "jesus" {
		t.Fatalf("speaker = %q, want jesus", resp.Verses[0].Segments[0].Speaker)
	}
}

func TestLookupBookOnlyExit1(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	stdout, stderr, code := runCLI(t, srv.URL, "John")
	if code != 1 {
		t.Fatalf("code = %d, want 1, stdout = %q stderr = %q", code, stdout, stderr)
	}
	if called {
		t.Fatal("must not call API for book-only reference")
	}
	if !strings.Contains(stderr, "pass a chapter") {
		t.Fatalf("stderr = %q, want pass a chapter", stderr)
	}
}

func TestReadLookupJohn316(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "read", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "For God so loved the world") {
		t.Fatalf("stdout missing WEB text:\n%s", stdout)
	}
}

func TestLookupAPI400Exit2(t *testing.T) {
	srv := newFixtureServer(t, http.StatusBadRequest, "error_400.json")
	_, stderr, code := runCLI(t, srv.URL, "John", "3:0")
	if code != 2 {
		t.Fatalf("code = %d, want 2, stderr = %q", code, stderr)
	}
}

func TestLookupSystemErrorExit3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(srv.Close)

	_, _, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 3 {
		t.Fatalf("code = %d, want 3", code)
	}
}

func TestNoArgsPrintsHelp(t *testing.T) {
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Fatalf("help missing Usage:\n%s", out.String())
	}
}

func TestTranslationFlagKJV(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "-t", "kjv", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(gotQuery, "translation=kjv") {
		t.Fatalf("query = %q, want translation=kjv", gotQuery)
	}
	if !strings.Contains(gotQuery, "segments=1") {
		t.Fatalf("query = %q, want segments=1", gotQuery)
	}
}

func TestLookupUsesConfigTranslation(t *testing.T) {
	writeXDGConfig(t, "translation = \"kjv\"\n")
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(gotQuery, "translation=kjv") {
		t.Fatalf("query = %q, want translation=kjv from config", gotQuery)
	}
}

func TestLookupEnvOverridesConfig(t *testing.T) {
	writeXDGConfig(t, "translation = \"kjv\"\n")
	t.Setenv("BIBLE_TRANSLATION", "asv")
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(gotQuery, "translation=asv") {
		t.Fatalf("query = %q, want translation=asv from env", gotQuery)
	}
}

func TestLookupFlagOverridesEnv(t *testing.T) {
	writeXDGConfig(t, "translation = \"kjv\"\n")
	t.Setenv("BIBLE_TRANSLATION", "asv")
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "-t", "web", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(gotQuery, "translation=web") {
		t.Fatalf("query = %q, want translation=web from flag", gotQuery)
	}
}

func TestLookupInvalidRedLetterEnvExit1(t *testing.T) {
	tests := []string{"not-a-bool", "yes", "1"}
	for _, val := range tests {
		t.Run(val, func(t *testing.T) {
			called := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called++
			}))
			t.Cleanup(srv.Close)

			isolateCmdEnv(t)
			t.Setenv("BIBLE_RED_LETTER", val)
			var out, errb bytes.Buffer
			code := ExecuteWith([]string{"--api-url", srv.URL, "John", "3:16"}, &out, &errb)
			if code != 1 {
				t.Fatalf("code = %d, want 1, stderr = %q", code, errb.String())
			}
			if called != 0 {
				t.Fatalf("fake server got %d requests, want 0", called)
			}
			if !strings.Contains(errb.String(), "BIBLE_RED_LETTER") {
				t.Fatalf("stderr = %q, want BIBLE_RED_LETTER", errb.String())
			}
		})
	}
}

func TestLookupUsesConfigAPIURL(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	writeXDGConfig(t, "api_url = \""+srv.URL+"\"\n")

	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if !called {
		t.Fatal("expected request to config api_url")
	}
}

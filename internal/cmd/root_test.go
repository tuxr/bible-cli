package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tuxr/bible-cli/internal/api"
	"github.com/tuxr/bible-cli/internal/tui"
)

func isolateCmdEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
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

func stubTUI(t *testing.T) *tui.Options {
	t.Helper()
	var got tui.Options
	orig := startTUI
	startTUI = func(o tui.Options) error {
		got = o
		return nil
	}
	t.Cleanup(func() { startTUI = orig })
	return &got
}

func TestNoArgsStartsTUI(t *testing.T) {
	isolateCmdEnv(t)
	got := stubTUI(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if got.Client == nil {
		t.Fatal("TUI not started")
	}
	if got.Ref != "" {
		t.Fatalf("ref = %q, want empty", got.Ref)
	}
	if got.ResumeBook != "" || got.ResumeChapter != 0 {
		t.Fatalf("resume = %s %d, want empty", got.ResumeBook, got.ResumeChapter)
	}
	if got.Translation != "web" {
		t.Fatalf("translation = %q", got.Translation)
	}
}

func TestNoArgsPassesResume(t *testing.T) {
	writeXDGConfig(t, "[resume]\nbook = \"JHN\"\nchapter = 3\n")
	got := stubTUI(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if got.ResumeBook != "JHN" || got.ResumeChapter != 3 {
		t.Fatalf("resume = %s %d", got.ResumeBook, got.ResumeChapter)
	}
	if got.Ref != "" {
		t.Fatalf("ref = %q, want empty so model uses resume", got.Ref)
	}
}

func TestTUICommandPassesRef(t *testing.T) {
	isolateCmdEnv(t)
	got := stubTUI(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"tui", "John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if got.Ref != "John 3:16" {
		t.Fatalf("ref = %q", got.Ref)
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

func TestVersionNoHTTP(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	orig := Version
	Version = "dev"
	t.Cleanup(func() { Version = orig })

	stdout, stderr, code := runCLI(t, srv.URL, "version")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if called {
		t.Fatal("version must not call API")
	}
	if strings.TrimSpace(stdout) != "dev" {
		t.Fatalf("stdout = %q, want dev", stdout)
	}
}

func TestInvalidColorOnVersionAndCompletion(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	for _, args := range [][]string{
		{"--color", "rainbow", "version"},
		{"version", "--color", "rainbow"},
		{"--color", "rainbow", "completion", "bash"},
		{"completion", "--color", "rainbow", "bash"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := runCLI(t, srv.URL, args...)
			if code != 1 {
				t.Fatalf("code = %d, want 1, stdout = %q stderr = %q", code, stdout, stderr)
			}
			if called {
				t.Fatal("must not call API")
			}
			if !strings.Contains(stderr, "invalid color") {
				t.Fatalf("stderr = %q", stderr)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestCompletionNoHTTP(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	stdout, stderr, code := runCLI(t, srv.URL, "completion", "bash")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q stdout = %q", code, stderr, stdout)
	}
	if called {
		t.Fatal("completion must not call API")
	}
	if !strings.Contains(stdout, "bible") {
		t.Fatalf("completion missing bible:\n%s", stdout)
	}
}

func TestConfigReservedNoHTTP(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "config")
	if code != 1 {
		t.Fatalf("code = %d, want 1, stderr = %q", code, stderr)
	}
	if called {
		t.Fatal("config must not be treated as a reference")
	}
	if !strings.Contains(stderr, "reserved") {
		t.Fatalf("stderr = %q, want reserved", stderr)
	}
}

func TestLookupUserAgentVersion(t *testing.T) {
	orig := Version
	Version = "4.5.6"
	t.Cleanup(func() { Version = orig })

	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.UserAgent()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(readFixture(t, "verse_john_3_16_segments.json"))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if ua != "bible-cli/4.5.6 (+https://github.com/tuxr/bible-cli)" {
		t.Fatalf("User-Agent = %q", ua)
	}
}

func TestPipedLookupSingleVerse(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "\x1b") {
		t.Fatalf("ANSI in piped lookup: %q", stdout)
	}
	if strings.Contains(stdout, "WEB") {
		t.Fatalf("translation suffix in piped lookup:\n%s", stdout)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %#v", lines)
	}
	if lines[0] != "John 3:16" {
		t.Fatalf("header = %q", lines[0])
	}
	if strings.Contains(lines[1], "	") || strings.HasPrefix(lines[1], "16") {
		t.Fatalf("single verse must be text only, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "For God so loved the world") {
		t.Fatalf("missing verse text: %q", lines[1])
	}
}

func TestPipedLookupColorAlwaysNoANSI(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "--color", "always", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "\x1b") {
		t.Fatalf("piped lookup must be ANSI-free even with --color always: %q", stdout)
	}
	if !strings.Contains(stdout, "For God so loved the world") {
		t.Fatalf("missing verse text: %q", stdout)
	}
}

func TestPipedLookupMultiVerse(t *testing.T) {
	body := []byte(`{
  "reference": "John 3:16-17",
  "translation": {"id": "web", "name": "World English Bible", "language": "en"},
  "verses": [
    {"verse": 16, "text": "For God so loved the world."},
    {"verse": 17, "text": "For God didn't send his Son into the world to judge the world."}
  ]
}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	stdout, stderr, code := runCLI(t, srv.URL, "John", "3:16-17")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	want := "John 3:16-17\n16	For God so loved the world.\n17	For God didn't send his Son into the world to judge the world.\n"
	if stdout != want {
		t.Fatalf("stdout = %q\nwant %q", stdout, want)
	}
}

func TestTTYLookupKeepsHeaderAndNumbers(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "--color", "never", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "John 3:16 (WEB)") {
		t.Fatalf("missing TTY header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "16  For God so loved the world") {
		t.Fatalf("missing numbered TTY line:\n%s", stdout)
	}
}

func TestLookupPipedNotJSON(t *testing.T) {
	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatalf("piped lookup must not auto-JSON:\n%s", stdout)
	}
}

func TestInvalidColorExit1(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "--color", "rainbow", "John", "3:16")
	if code != 1 {
		t.Fatalf("code = %d, want 1, stderr = %q", code, stderr)
	}
	if called != 0 {
		t.Fatalf("requests = %d, want 0", called)
	}
	if !strings.Contains(stderr, "invalid color") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestColorNO_COLORDisablesOnTTY(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	isolateCmdEnv(t)
	t.Setenv("NO_COLOR", "1")
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"--api-url", srv.URL, "John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("ANSI with NO_COLOR: %q", out.String())
	}
}

func TestColorFORCE_COLOREnablesOnTTY(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	isolateCmdEnv(t)
	t.Setenv("FORCE_COLOR", "1")
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"--api-url", srv.URL, "John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "\x1b") {
		t.Fatalf("expected ANSI with FORCE_COLOR, got %q", out.String())
	}
}

func TestColorNO_COLORWinsOverFORCE_COLOR(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	isolateCmdEnv(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"--api-url", srv.URL, "John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("NO_COLOR must win: %q", out.String())
	}
}

func TestColorFlagWinsOverEnv(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	isolateCmdEnv(t)
	t.Setenv("NO_COLOR", "1")
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"--api-url", srv.URL, "--color", "always", "John", "3:16"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "\x1b") {
		t.Fatalf("explicit --color=always must win, got %q", out.String())
	}
}

func TestThemeDefaultsToDark(t *testing.T) {
	origTTY := writerIsTTY
	writerIsTTY = func(io.Writer) bool { return true }
	t.Cleanup(func() { writerIsTTY = origTTY })

	srv := newFixtureServer(t, http.StatusOK, "verse_john_3_16_segments.json")
	stdout, stderr, code := runCLI(t, srv.URL, "--color", "always", "John", "3:16")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "\x1b[38;2;195;30;58m") {
		t.Fatalf("expected dark red-letter ANSI, got %q", stdout)
	}
	if strings.Contains(stdout, "\x1b[38;2;155;27;48m") {
		t.Fatalf("light red-letter ANSI present: %q", stdout)
	}
}

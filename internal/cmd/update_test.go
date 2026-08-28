package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tuxr/bible-cli/internal/ghrel"
)

func makeTGZ(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func stubExecutable(t *testing.T, path string) {
	t.Helper()
	orig := osExecutable
	osExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { osExecutable = orig })
}

func stubPlatform(t *testing.T, goos, goarch string) {
	t.Helper()
	o, a := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = goos, goarch
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = o, a
	})
}

func stubGitHub(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	origAPI, origWeb := githubAPI, githubWeb
	githubAPI, githubWeb = srv.URL, srv.URL
	t.Cleanup(func() {
		githubAPI, githubWeb = origAPI, origWeb
	})
	return srv
}

func TestReservedNamesIncludeUpdateUninstall(t *testing.T) {
	for _, name := range []string{
		"tui", "read", "search", "translations", "books", "random",
		"version", "help", "completion", "config", "update", "uninstall",
	} {
		if _, ok := reservedRefs[name]; !ok {
			t.Errorf("reservedRefs missing %q", name)
		}
	}
}

func TestReservedUpdateUninstallNotLookup(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "bible-cli")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	t.Setenv("GOBIN", filepath.Join(dir, "gobin"))
	t.Setenv("GOPATH", filepath.Join(dir, "gopath"))

	gh := stubGitHub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	_ = gh

	for _, args := range [][]string{{"update"}, {"uninstall", "--prefix", filepath.Join(dir, "missing-prefix")}} {
		called = false
		_, stderr, code := runCLI(t, srv.URL, args...)
		if called {
			t.Fatalf("%v hit bible-api, stderr=%q", args, stderr)
		}
		if code == 0 {
			t.Fatalf("%v code = 0, want non-zero (not a scripture lookup)", args)
		}
		if code == 2 {
			t.Fatalf("%v code = 2 (looks like API not-found), stderr=%q", args, stderr)
		}
	}
}

func TestUninstallRefusesBible(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bible")
	if err := os.WriteFile(exe, []byte("kjv"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"uninstall", "--prefix", dir}, &out, &errb)
	if code != 1 {
		t.Fatalf("code = %d, want 1, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "refusing") {
		t.Fatalf("stderr = %q, want refusing", errb.String())
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "kjv" {
		t.Fatalf("deleted bible binary: %q", got)
	}
}

func TestUninstallRemovesBibleCLI(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bible-cli")
	if err := os.WriteFile(exe, []byte("cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"uninstall", "--prefix", dir}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	if _, err := os.Stat(exe); !os.IsNotExist(err) {
		t.Fatalf("bible-cli still present: %v", err)
	}
}

func TestUpdateChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bible-cli")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	stubPlatform(t, "linux", "amd64")
	t.Setenv("GOBIN", filepath.Join(dir, "gobin"))
	t.Setenv("GOPATH", filepath.Join(dir, "gopath"))

	archive := ghrel.ArchiveName("v0.2.0", "linux", "amd64")
	tgz := makeTGZ(t, "bible-cli", []byte("new-bin"))
	sums := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  " + archive + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/tuxr/bible-cli/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v0.2.0"}`)
	})
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/"+archive, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tgz)
	})
	stubGitHub(t, mux)

	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"update"}, &out, &errb)
	if code != 3 {
		t.Fatalf("code = %d, want 3, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "checksum") {
		t.Fatalf("stderr = %q, want checksum", errb.String())
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("binary replaced after checksum fail: %q", got)
	}
}

func TestUpdateNetworkErrorExit3(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bible-cli")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	stubPlatform(t, "linux", "amd64")
	t.Setenv("GOBIN", filepath.Join(dir, "gobin"))
	t.Setenv("GOPATH", filepath.Join(dir, "gopath"))
	stubGitHub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"update"}, &out, &errb)
	if code != 3 {
		t.Fatalf("code = %d, want 3, stderr = %q", code, errb.String())
	}
}

func TestUpdateRefusesGoInstall(t *testing.T) {
	gobin := t.TempDir()
	exe := filepath.Join(gobin, "bible-cli")
	if err := os.WriteFile(exe, []byte("go-install"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	t.Setenv("GOBIN", gobin)
	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"update"}, &out, &errb)
	if code != 1 {
		t.Fatalf("code = %d, want 1, stderr = %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "go install") {
		t.Fatalf("stderr = %q, want go install hint", errb.String())
	}
}

func TestUpdateReplacesReleaseBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "bible-cli")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubExecutable(t, exe)
	stubPlatform(t, "linux", "amd64")
	t.Setenv("GOBIN", filepath.Join(dir, "gobin"))
	t.Setenv("GOPATH", filepath.Join(dir, "gopath"))

	archive := ghrel.ArchiveName("v0.2.0", "linux", "amd64")
	tgz := makeTGZ(t, "bible-cli", []byte("new-bin"))
	sums := sha256Hex(tgz) + "  " + archive + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/tuxr/bible-cli/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v0.2.0"}`)
	})
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/"+archive, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tgz)
	})
	stubGitHub(t, mux)

	isolateCmdEnv(t)
	var out, errb bytes.Buffer
	code := ExecuteWith([]string{"update"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errb.String())
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-bin" {
		t.Fatalf("binary = %q, want new-bin", got)
	}
}

func TestInstallScriptChecksumFail(t *testing.T) {
	if !ghrel.Supported(runtime.GOOS, runtime.GOARCH) {
		t.Skip("unsupported platform")
	}
	archive := ghrel.ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH)
	tgz := makeTGZ(t, "bible-cli", []byte("payload"))
	sums := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  " + archive + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	mux.HandleFunc("/tuxr/bible-cli/releases/download/v0.2.0/"+archive, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tgz)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prefix := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "install.sh")
	cmd := exec.Command("sh", script, "--prefix", prefix, "--version", "v0.2.0")
	cmd.Env = append(os.Environ(), "GITHUB="+srv.URL, "PREFIX="+prefix)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum failure, output:\n%s", out)
	}
	if !strings.Contains(string(out), "checksum") {
		t.Fatalf("output = %s, want checksum", out)
	}
	if _, err := os.Stat(filepath.Join(prefix, "bible-cli")); !os.IsNotExist(err) {
		t.Fatalf("installed bible-cli after checksum fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "bible")); !os.IsNotExist(err) {
		t.Fatalf("wrote bible binary: %v", err)
	}
}

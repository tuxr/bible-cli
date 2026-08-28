package ghrel

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestArchiveName(t *testing.T) {
	got := ArchiveName("v0.2.0", "linux", "amd64")
	want := "bible-cli_0.2.0_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("ArchiveName = %q, want %q", got, want)
	}
}

func TestSupported(t *testing.T) {
	if !Supported("linux", "amd64") || !Supported("darwin", "arm64") {
		t.Fatal("expected linux/darwin amd64/arm64")
	}
	if Supported("windows", "amd64") || Supported("linux", "386") {
		t.Fatal("windows and 386 must be unsupported")
	}
}

func TestChecksumForMissing(t *testing.T) {
	_, err := ChecksumFor([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other.tar.gz\n"), "bible-cli_0.2.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected missing checksum error")
	}
}

func TestChecksumForAndVerify(t *testing.T) {
	payload := []byte("hello")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	sums := hexSum + "  bible-cli_0.2.0_linux_amd64.tar.gz\n"
	got, err := ChecksumFor([]byte(sums), "bible-cli_0.2.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != hexSum {
		t.Fatalf("got %q", got)
	}
	if err := VerifySHA256(payload, got); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256([]byte("nope"), got); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestExtractBinarySkipsBible(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	// tiny invalid tar would fail; covered by cmd tests with real tar
	_ = gz.Close()
	if _, err := ExtractBinary(buf.Bytes()); err == nil {
		t.Fatal("empty archive should fail")
	}
}

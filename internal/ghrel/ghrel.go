package ghrel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultAPI is the GitHub API origin.
	DefaultAPI = "https://api.github.com"
	// DefaultWeb is the GitHub download origin.
	DefaultWeb = "https://github.com"
	// Repo is owner/name for GitHub Releases.
	Repo = "tuxr/bible-cli"
	// BinaryName is the installed command name.
	BinaryName = "bible-cli"
	// ChecksumsName is the GoReleaser checksums asset.
	ChecksumsName = "checksums.txt"

	httpTimeout = 30 * time.Second
	maxAsset    = 64 << 20
)

// Client fetches GitHub Release metadata and assets.
type Client struct {
	API       string
	Web       string
	HTTP      *http.Client
	UserAgent string
}

func (c *Client) apiBase() string {
	u := strings.TrimRight(strings.TrimSpace(c.API), "/")
	if u == "" {
		return DefaultAPI
	}
	return u
}

func (c *Client) webBase() string {
	u := strings.TrimRight(strings.TrimSpace(c.Web), "/")
	if u == "" {
		return DefaultWeb
	}
	return u
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: httpTimeout}
}

func (c *Client) ua() string {
	if v := strings.TrimSpace(c.UserAgent); v != "" {
		return v
	}
	return "bible-cli/dev (+https://github.com/tuxr/bible-cli)"
}

// LatestTag returns the latest release tag (e.g. v0.2.0).
func (c *Client) LatestTag(ctx context.Context) (string, error) {
	u := c.apiBase() + "/repos/" + Repo + "/releases/latest"
	body, err := c.get(ctx, u, "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	var parsed struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("github release: %w", err)
	}
	tag := strings.TrimSpace(parsed.TagName)
	if tag == "" {
		return "", fmt.Errorf("github release: missing tag_name")
	}
	return tag, nil
}

// AssetURL is the GitHub download URL for an asset on tag.
func (c *Client) AssetURL(tag, name string) string {
	return c.webBase() + "/" + Repo + "/releases/download/" + tag + "/" + name
}

// Download fetches a URL and returns the body.
func (c *Client) Download(ctx context.Context, rawURL string) ([]byte, error) {
	return c.get(ctx, rawURL, "*/*")
}

func (c *Client) get(ctx context.Context, rawURL, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua())
	req.Header.Set("Accept", accept)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAsset+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAsset {
		return nil, fmt.Errorf("github release: response too large")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github release: HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// ArchiveName is bible-cli_{version}_{os}_{arch}.tar.gz (version without v).
func ArchiveName(version, goos, goarch string) string {
	return BinaryName + "_" + strings.TrimPrefix(version, "v") + "_" + goos + "_" + goarch + ".tar.gz"
}

// Supported reports linux/darwin × amd64/arm64.
func Supported(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin":
	default:
		return false
	}
	switch goarch {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

// ChecksumFor returns the SHA-256 hex for filename from checksums.txt.
func ChecksumFor(checksums []byte, filename string) (string, error) {
	base := filepath.Base(filename)
	for _, raw := range strings.Split(string(checksums), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) != base {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 {
			return "", fmt.Errorf("checksum malformed for %s", base)
		}
		for _, r := range sum {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return "", fmt.Errorf("checksum malformed for %s", base)
			}
		}
		return sum, nil
	}
	return "", fmt.Errorf("checksum missing for %s", base)
}

// VerifySHA256 fails closed unless data matches wantHex.
func VerifySHA256(data []byte, wantHex string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != strings.ToLower(strings.TrimSpace(wantHex)) {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

// ExtractBinary reads bible-cli from a .tar.gz. It never returns a file named bible.
func ExtractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base == "bible" {
			continue
		}
		if base != BinaryName {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxAsset+1))
		if err != nil {
			return nil, fmt.Errorf("archive: %w", err)
		}
		if len(data) > maxAsset {
			return nil, fmt.Errorf("archive: bible-cli too large")
		}
		return data, nil
	}
	return nil, fmt.Errorf("archive missing %s", BinaryName)
}

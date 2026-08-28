package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tuxr/bible-cli/internal/config"
	"github.com/tuxr/bible-cli/internal/ghrel"
)

var (
	osExecutable  = os.Executable
	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
	githubAPI     = ghrel.DefaultAPI
	githubWeb     = ghrel.DefaultWeb
)

func defaultPrefix() string {
	if p := strings.TrimSpace(os.Getenv("PREFIX")); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "bin")
}

func (o *options) addSelfCommands(root *cobra.Command) {
	update := &cobra.Command{
		Use:   "update",
		Short: "Replace this binary with the latest GitHub Release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context())
		},
	}
	var prefix string
	var purge bool
	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the bible-cli binary from the install prefix",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := strings.TrimSpace(prefix)
			if p == "" {
				p = defaultPrefix()
			}
			return runUninstall(p, purge)
		},
	}
	uninstall.Flags().StringVar(&prefix, "prefix", "", "install prefix (default $PREFIX or ~/.local/bin)")
	uninstall.Flags().BoolVar(&purge, "purge", false, "also delete XDG config ($XDG_CONFIG_HOME/bible)")
	root.AddCommand(update, uninstall)
}

func runUpdate(ctx context.Context) error {
	if !ghrel.Supported(runtimeGOOS, runtimeGOARCH) {
		return usagef("unsupported platform %s/%s (linux|darwin amd64|arm64)", runtimeGOOS, runtimeGOARCH)
	}
	exe, err := resolvedExecutable()
	if err != nil {
		return err
	}
	if err := refuseBiblePath(exe); err != nil {
		return err
	}
	if filepath.Base(exe) != ghrel.BinaryName {
		return usagef("expected %s, got %s", ghrel.BinaryName, exe)
	}
	if isGoInstall(exe) {
		return usagef("this binary is not a GitHub Release install; re-run go install or scripts/install.sh")
	}
	dir := filepath.Dir(exe)
	if err := writableDir(dir); err != nil {
		return usagef("cannot replace %s: %s", exe, err.Error())
	}

	client := &ghrel.Client{
		API:       githubAPI,
		Web:       githubWeb,
		UserAgent: "bible-cli/" + Version + " (+https://github.com/tuxr/bible-cli)",
	}
	tag, err := client.LatestTag(ctx)
	if err != nil {
		return systemf("%s", err.Error())
	}
	archive := ghrel.ArchiveName(tag, runtimeGOOS, runtimeGOARCH)
	sumsBody, err := client.Download(ctx, client.AssetURL(tag, ghrel.ChecksumsName))
	if err != nil {
		return systemf("%s", err.Error())
	}
	want, err := ghrel.ChecksumFor(sumsBody, archive)
	if err != nil {
		return systemf("%s", err.Error())
	}
	tgz, err := client.Download(ctx, client.AssetURL(tag, archive))
	if err != nil {
		return systemf("%s", err.Error())
	}
	if err := ghrel.VerifySHA256(tgz, want); err != nil {
		return systemf("%s", err.Error())
	}
	bin, err := ghrel.ExtractBinary(tgz)
	if err != nil {
		return systemf("%s", err.Error())
	}
	if err := replaceFile(exe, bin); err != nil {
		return systemf("%s", err.Error())
	}
	return nil
}

func runUninstall(prefix string, purge bool) error {
	if strings.TrimSpace(prefix) == "" {
		return usagef("missing install prefix")
	}
	exe, err := resolvedExecutable()
	if err != nil {
		return err
	}
	if err := refuseBiblePath(exe); err != nil {
		return err
	}
	dest := filepath.Join(prefix, ghrel.BinaryName)
	if err := refuseBiblePath(dest); err != nil {
		return err
	}
	if filepath.Base(dest) != ghrel.BinaryName {
		return usagef("refusing to delete %s", dest)
	}
	if err := os.Remove(dest); err != nil {
		if os.IsNotExist(err) {
			return usagef("not installed: %s", dest)
		}
		return systemf("%s", err.Error())
	}
	if !purge {
		return nil
	}
	path, err := config.Path()
	if err != nil {
		return systemf("%s", err.Error())
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "bible" {
		return usagef("refusing to delete %s", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return systemf("%s", err.Error())
	}
	return nil
}

func resolvedExecutable() (string, error) {
	exe, err := osExecutable()
	if err != nil {
		return "", systemf("executable: %s", err.Error())
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", systemf("executable: %s", err.Error())
	}
	return exe, nil
}

func refuseBiblePath(p string) error {
	if filepath.Base(p) == "bible" {
		return usagef("refusing to delete %s", p)
	}
	return nil
}

func isGoInstall(exe string) bool {
	slash := filepath.ToSlash(exe)
	if strings.Contains(slash, "/pkg/mod/") {
		return true
	}
	dir := filepath.Dir(exe)
	if g := strings.TrimSpace(os.Getenv("GOBIN")); g != "" {
		if sameDir(dir, g) {
			return true
		}
	}
	gopath := strings.TrimSpace(os.Getenv("GOPATH"))
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return false
		}
		gopath = filepath.Join(home, "go")
	}
	for _, p := range filepath.SplitList(gopath) {
		if sameDir(dir, filepath.Join(p, "bin")) {
			return true
		}
	}
	return false
}

func sameDir(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return aa == bb
}

func writableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".bible-cli-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func replaceFile(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, "bible-cli-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}

func systemf(format string, args ...any) error {
	return &systemError{msg: fmt.Sprintf(format, args...)}
}

type systemError struct {
	msg string
}

func (e *systemError) Error() string { return e.msg }

package main

import (
	"os"

	"github.com/tuxr/bible-cli/internal/cmd"
)

// version is injected by GoReleaser via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cmd.Execute(version))
}

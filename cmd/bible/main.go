package main

import (
	"os"

	"github.com/tuxr/bible-cli/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}

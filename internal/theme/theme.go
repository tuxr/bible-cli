package theme

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette is a named set of UI colors for the bible TUI.
type Palette struct {
	Name      string
	Bg        string
	Fg        string
	FgMuted   string
	FgDim     string
	Accent    string
	Border    string
	RedLetter string
	Error     string
	Warning   string
	FooterBg  string
	FooterFg  string
}

// Dark is the default cool-dark palette (not pure black).
var Dark = Palette{
	Name:      "dark",
	Bg:        "#2E3440",
	Fg:        "#ECEFF4",
	FgMuted:   "#D8DEE9",
	FgDim:     "#7B88A1",
	Accent:    "#88C0D0",
	Border:    "#4C566A",
	RedLetter: "#C41E3A",
	Error:     "#BF616A",
	Warning:   "#EBCB8B",
	FooterBg:  "#3B4252",
	FooterFg:  "#D8DEE9",
}

// Light is a high-contrast light palette with crimson red-letter text.
var Light = Palette{
	Name:      "light",
	Bg:        "#ECEFF4",
	Fg:        "#2E3440",
	FgMuted:   "#4C566A",
	FgDim:     "#7B88A1",
	Accent:    "#5E81AC",
	Border:    "#D8DEE9",
	RedLetter: "#9B1B30",
	Error:     "#A5122A",
	Warning:   "#B8832A",
	FooterBg:  "#E5E9F0",
	FooterFg:  "#3B4252",
}

// detectDarkBackground reports (isDark, ok). When ok is false, Lookup("auto")
// falls back to Dark. Tests replace this; production uses queryDarkBackground.
var detectDarkBackground = queryDarkBackground

func queryDarkBackground() (dark bool, ok bool) {
	if os.Stdout == nil {
		return false, false
	}
	st, err := os.Stdout.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false, false
	}
	return lipgloss.HasDarkBackground(), true
}

// Lookup returns a palette by name.
// "dark" and "" are the default cool-dark palette.
// "light" is the light palette.
// "auto" resolves to dark or light via lipgloss.HasDarkBackground when a
// terminal is available; otherwise it falls back to dark.
// The returned Name is never "auto".
func Lookup(name string) (Palette, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "dark":
		return Dark, nil
	case "light":
		return Light, nil
	case "auto":
		return resolveAuto(), nil
	default:
		return Palette{}, fmt.Errorf("unknown theme %q", name)
	}
}

func resolveAuto() Palette {
	if detectDarkBackground != nil {
		if dark, ok := detectDarkBackground(); ok {
			if dark {
				return Dark
			}
			return Light
		}
	}
	return Dark
}

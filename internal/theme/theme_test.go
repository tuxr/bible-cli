package theme

import (
	"strings"
	"testing"
)

func TestLookupDark(t *testing.T) {
	p, err := Lookup("dark")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "dark" {
		t.Fatalf("Name = %q, want dark", p.Name)
	}
	if p.RedLetter == "" {
		t.Fatal("RedLetter empty")
	}
	if p.Fg == "" {
		t.Fatal("Fg empty")
	}
}

func TestLookupLight(t *testing.T) {
	p, err := Lookup("light")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "light" {
		t.Fatalf("Name = %q, want light", p.Name)
	}
	if p.RedLetter == "" {
		t.Fatal("RedLetter empty")
	}
	if p.Fg == "" {
		t.Fatal("Fg empty")
	}
}

func TestLookupNordfoxErrors(t *testing.T) {
	_, err := Lookup("nordfox")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLookupAutoResolvesDarkOrLight(t *testing.T) {
	p, err := Lookup("auto")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "dark" && p.Name != "light" {
		t.Fatalf("Name = %q, want dark or light", p.Name)
	}
}

func TestLookupEmptyIsDark(t *testing.T) {
	got, err := Lookup("")
	if err != nil {
		t.Fatal(err)
	}
	want, err := Lookup("dark")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Lookup(\"\") = %+v, want dark %+v", got, want)
	}
	if got.Name != "dark" {
		t.Fatalf("Name = %q, want dark", got.Name)
	}
}

func TestLookupAutoNameNeverAuto(t *testing.T) {
	orig := detectDarkBackground
	t.Cleanup(func() { detectDarkBackground = orig })

	cases := []struct {
		name string
		fn   func() (bool, bool)
		want string
	}{
		{"dark-ok", func() (bool, bool) { return true, true }, "dark"},
		{"light-ok", func() (bool, bool) { return false, true }, "light"},
		{"unavailable", func() (bool, bool) { return false, false }, "dark"},
		{"nil", nil, "dark"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detectDarkBackground = tc.fn
			p, err := Lookup("auto")
			if err != nil {
				t.Fatal(err)
			}
			if p.Name != tc.want {
				t.Fatalf("Name = %q, want %q", p.Name, tc.want)
			}
			if p.Name == "auto" {
				t.Fatal("Name must not be auto")
			}
		})
	}
}

func TestPalettesHaveRequiredFields(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		p, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		checkRequired(t, p)
	}
}

func TestDarkBackgroundNotPureBlack(t *testing.T) {
	p, err := Lookup("dark")
	if err != nil {
		t.Fatal(err)
	}
	bg := strings.ToLower(strings.TrimPrefix(p.Bg, "#"))
	if bg == "000" || bg == "000000" {
		t.Fatalf("dark Bg = %q, must not be pure black", p.Bg)
	}
}

func TestUnknownThemeErrors(t *testing.T) {
	_, err := Lookup("sepia")
	if err == nil {
		t.Fatal("expected error")
	}
}

func checkRequired(t *testing.T, p Palette) {
	t.Helper()
	fields := []struct {
		name, val string
	}{
		{"Name", p.Name},
		{"Bg", p.Bg},
		{"Fg", p.Fg},
		{"FgMuted", p.FgMuted},
		{"FgDim", p.FgDim},
		{"Accent", p.Accent},
		{"Border", p.Border},
		{"RedLetter", p.RedLetter},
		{"Error", p.Error},
		{"Warning", p.Warning},
		{"FooterBg", p.FooterBg},
		{"FooterFg", p.FooterFg},
	}
	for _, f := range fields {
		if f.val == "" {
			t.Fatalf("%s empty on palette %q", f.name, p.Name)
		}
	}
}

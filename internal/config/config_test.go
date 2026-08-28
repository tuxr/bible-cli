package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("BIBLE_TRANSLATION", "")
	t.Setenv("BIBLE_THEME", "")
	t.Setenv("BIBLE_API_URL", "")
	t.Setenv("BIBLE_RED_LETTER", "")
	return dir
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	cfgDir := filepath.Join(dir, "bible")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPathUsesXDGConfigHome(t *testing.T) {
	dir := isolate(t)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "bible", "config.toml")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadDefaultsWhenNoFileOrEnv(t *testing.T) {
	isolate(t)
	cfg, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Translation != DefaultTranslation {
		t.Fatalf("translation = %q, want %q", cfg.Translation, DefaultTranslation)
	}
	if cfg.Theme != DefaultTheme {
		t.Fatalf("theme = %q, want %q", cfg.Theme, DefaultTheme)
	}
	if cfg.APIURL != DefaultAPIURL {
		t.Fatalf("api_url = %q, want %q", cfg.APIURL, DefaultAPIURL)
	}
	if cfg.RedLetter != DefaultRedLetter {
		t.Fatalf("red_letter = %v, want %v", cfg.RedLetter, DefaultRedLetter)
	}
	if cfg.Resume.Book != "" || cfg.Resume.Chapter != 0 {
		t.Fatalf("resume = %+v, want empty", cfg.Resume)
	}
}

func TestPrecedenceFlagOverEnvOverFileOverDefault(t *testing.T) {
	const fileTOML = `
translation = "kjv"
theme = "light"
api_url = "https://file.example"
red_letter = false

[resume]
book = "JHN"
chapter = 3
`

	tests := []struct {
		name string
		file string
		env  map[string]string
		flag Flags
		want Config
	}{
		{
			name: "defaults",
			want: Config{
				Translation: DefaultTranslation,
				Theme:       DefaultTheme,
				APIURL:      DefaultAPIURL,
				RedLetter:   DefaultRedLetter,
			},
		},
		{
			name: "file over default",
			file: fileTOML,
			want: Config{
				Translation: "kjv",
				Theme:       "light",
				APIURL:      "https://file.example",
				RedLetter:   false,
				Resume:      Resume{Book: "JHN", Chapter: 3},
			},
		},
		{
			name: "env over file",
			file: fileTOML,
			env: map[string]string{
				"BIBLE_TRANSLATION": "asv",
				"BIBLE_THEME":       "dark",
				"BIBLE_API_URL":     "https://env.example",
				"BIBLE_RED_LETTER":  "true",
			},
			want: Config{
				Translation: "asv",
				Theme:       "dark",
				APIURL:      "https://env.example",
				RedLetter:   true,
				Resume:      Resume{Book: "JHN", Chapter: 3},
			},
		},
		{
			name: "flag over env over file",
			file: fileTOML,
			env: map[string]string{
				"BIBLE_TRANSLATION": "asv",
				"BIBLE_THEME":       "dark",
				"BIBLE_API_URL":     "https://env.example",
				"BIBLE_RED_LETTER":  "false",
			},
			flag: Flags{
				Translation:    "web",
				Theme:          "auto",
				APIURL:         "https://flag.example",
				RedLetter:      true,
				HasTranslation: true,
				HasTheme:       true,
				HasAPIURL:      true,
				HasRedLetter:   true,
			},
			want: Config{
				Translation: "web",
				Theme:       "auto",
				APIURL:      "https://flag.example",
				RedLetter:   true,
				Resume:      Resume{Book: "JHN", Chapter: 3},
			},
		},
		{
			name: "partial file keeps other defaults",
			file: `translation = "wlc"` + "\n",
			want: Config{
				Translation: "wlc",
				Theme:       DefaultTheme,
				APIURL:      DefaultAPIURL,
				RedLetter:   DefaultRedLetter,
			},
		},
		{
			name: "empty env does not override file",
			file: `translation = "kjv"` + "\n",
			env:  map[string]string{"BIBLE_TRANSLATION": "  "},
			want: Config{
				Translation: "kjv",
				Theme:       DefaultTheme,
				APIURL:      DefaultAPIURL,
				RedLetter:   DefaultRedLetter,
			},
		},
		{
			name: "unset flags do not clobber env",
			file: `translation = "kjv"` + "\n",
			env:  map[string]string{"BIBLE_TRANSLATION": "asv"},
			flag: Flags{Translation: DefaultTranslation},
			want: Config{
				Translation: "asv",
				Theme:       DefaultTheme,
				APIURL:      DefaultAPIURL,
				RedLetter:   DefaultRedLetter,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolate(t)
			if tc.file != "" {
				writeConfig(t, dir, tc.file)
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := Load(tc.flag)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, dir, "translation = [\n")
	_, err := Load(Flags{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Fatalf("err = %v, want config prefix", err)
	}
}

func TestSaveResumeWritesAndPreserves(t *testing.T) {
	dir := isolate(t)
	writeConfig(t, dir, "translation = \"kjv\"\ntheme = \"light\"\n")
	if err := SaveResume("JHN", 3); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Translation != "kjv" || cfg.Theme != "light" {
		t.Fatalf("preserved fields lost: %+v", cfg)
	}
	if cfg.Resume.Book != "JHN" || cfg.Resume.Chapter != 3 {
		t.Fatalf("resume = %+v", cfg.Resume)
	}

	isolate(t)
	if err := SaveResume("PSA", 23); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resume != (Resume{Book: "PSA", Chapter: 23}) {
		t.Fatalf("resume = %+v", cfg.Resume)
	}
	if cfg.Translation != DefaultTranslation {
		t.Fatalf("translation = %q", cfg.Translation)
	}
}

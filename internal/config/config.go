package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultTranslation = "web"
	DefaultTheme       = "dark"
	DefaultAPIURL      = "https://bible-api.dws-cloud.com"
	DefaultRedLetter   = true

	appDir   = "bible"
	fileName = "config.toml"
)

// Config is the resolved bible-cli configuration.
type Config struct {
	Translation string
	Theme       string
	APIURL      string
	RedLetter   bool
	Resume      Resume
}

// Resume is the last-read location persisted under [resume].
type Resume struct {
	Book    string
	Chapter int
}

// Flags are CLI overlays. Has* must be true for a field to win (cobra Changed).
type Flags struct {
	Translation string
	Theme       string
	APIURL      string
	RedLetter   bool

	HasTranslation bool
	HasTheme       bool
	HasAPIURL      bool
	HasRedLetter   bool
}

type fileConfig struct {
	Translation string
	Theme       string
	APIURL      string
	RedLetter   *bool
	Resume      Resume
}

// Path is $XDG_CONFIG_HOME/bible/config.toml (via os.UserConfigDir).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, fileName), nil
}

// Load applies precedence: flag > env > config file > default.
func Load(flags Flags) (Config, error) {
	cfg := Config{
		Translation: DefaultTranslation,
		Theme:       DefaultTheme,
		APIURL:      DefaultAPIURL,
		RedLetter:   DefaultRedLetter,
	}
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		parsed, err := parseTOML(data)
		if err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", path, err)
		}
		applyFile(&cfg, parsed)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	applyEnv(&cfg)
	applyFlags(&cfg, flags)
	return cfg, nil
}

// SaveResume writes [resume] to the XDG config file, preserving other keys.
func SaveResume(book string, chapter int) error {
	path, err := Path()
	if err != nil {
		return err
	}
	var parsed fileConfig
	data, err := os.ReadFile(path)
	if err == nil {
		parsed, err = parseTOML(data)
		if err != nil {
			return fmt.Errorf("config: %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parsed.Resume.Book = book
	parsed.Resume.Chapter = chapter
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, marshalConfig(parsed), 0o644)
}

func applyFile(cfg *Config, parsed fileConfig) {
	if v := strings.TrimSpace(parsed.Translation); v != "" {
		cfg.Translation = v
	}
	if v := strings.TrimSpace(parsed.Theme); v != "" {
		cfg.Theme = v
	}
	if v := strings.TrimSpace(parsed.APIURL); v != "" {
		cfg.APIURL = v
	}
	if parsed.RedLetter != nil {
		cfg.RedLetter = *parsed.RedLetter
	}
	if v := strings.TrimSpace(parsed.Resume.Book); v != "" {
		cfg.Resume.Book = v
	}
	if parsed.Resume.Chapter != 0 {
		cfg.Resume.Chapter = parsed.Resume.Chapter
	}
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("BIBLE_TRANSLATION")); v != "" {
		cfg.Translation = v
	}
	if v := strings.TrimSpace(os.Getenv("BIBLE_THEME")); v != "" {
		cfg.Theme = v
	}
	if v := strings.TrimSpace(os.Getenv("BIBLE_API_URL")); v != "" {
		cfg.APIURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BIBLE_RED_LETTER")); v != "" {
		if b, err := parseBoolEnv(v); err == nil {
			cfg.RedLetter = b
		}
	}
}

func parseBoolEnv(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", s)
	}
}

func applyFlags(cfg *Config, flags Flags) {
	if flags.HasTranslation {
		cfg.Translation = flags.Translation
	}
	if flags.HasTheme {
		cfg.Theme = flags.Theme
	}
	if flags.HasAPIURL {
		cfg.APIURL = flags.APIURL
	}
	if flags.HasRedLetter {
		cfg.RedLetter = flags.RedLetter
	}
}

func parseTOML(data []byte) (fileConfig, error) {
	var out fileConfig
	section := ""
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return fileConfig{}, fmt.Errorf("line %d: invalid table", i+1)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return fileConfig{}, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if err := assign(&out, section, key, val); err != nil {
			return fileConfig{}, fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return out, nil
}

func assign(out *fileConfig, section, key, raw string) error {
	v, err := parseValue(raw)
	if err != nil {
		return err
	}
	switch section {
	case "":
		switch key {
		case "translation":
			s, err := asString(v)
			if err != nil {
				return err
			}
			out.Translation = s
		case "theme":
			s, err := asString(v)
			if err != nil {
				return err
			}
			out.Theme = s
		case "api_url":
			s, err := asString(v)
			if err != nil {
				return err
			}
			out.APIURL = s
		case "red_letter":
			b, ok := v.(bool)
			if !ok {
				return fmt.Errorf("red_letter: want bool")
			}
			out.RedLetter = &b
		default:
			return nil
		}
	case "resume":
		switch key {
		case "book":
			s, err := asString(v)
			if err != nil {
				return err
			}
			out.Resume.Book = s
		case "chapter":
			n, ok := v.(int)
			if !ok {
				return fmt.Errorf("chapter: want integer")
			}
			out.Resume.Chapter = n
		default:
			return nil
		}
	default:
		return nil
	}
	return nil
}

func parseValue(s string) (any, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if len(s) >= 2 && s[0] == '"' {
		return strconv.Unquote(s)
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	return nil, fmt.Errorf("invalid value %q", s)
}

func asString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("want string")
	}
	return s, nil
}

func marshalConfig(parsed fileConfig) []byte {
	var b strings.Builder
	writeStr := func(k, v string) {
		if v == "" {
			return
		}
		fmt.Fprintf(&b, "%s = %s\n", k, strconv.Quote(v))
	}
	writeStr("translation", parsed.Translation)
	writeStr("theme", parsed.Theme)
	writeStr("api_url", parsed.APIURL)
	if parsed.RedLetter != nil {
		fmt.Fprintf(&b, "red_letter = %v\n", *parsed.RedLetter)
	}
	if parsed.Resume.Book != "" || parsed.Resume.Chapter != 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("[resume]\n")
		writeStr("book", parsed.Resume.Book)
		fmt.Fprintf(&b, "chapter = %d\n", parsed.Resume.Chapter)
	}
	return []byte(b.String())
}

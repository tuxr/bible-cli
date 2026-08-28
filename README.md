# bible-cli

Terminal Bible reader and CLI for [bible-api](https://bible-api.dws-cloud.com),
the same API used by [bible-web](https://bible.dws-cloud.com).

`bible` opens a full-screen chapter reader. `bible John 3:16` prints a verse.
Default translation is WEB.

Author: [tuxr](https://github.com/tuxr). License: [MIT](LICENSE).

## Install

Requires Go 1.26+:

```bash
go install github.com/tuxr/bible-cli/cmd/bible@latest
```

GitHub Releases also publish static `bible` binaries for Linux and macOS
(amd64 and arm64). Download an archive from
[Releases](https://github.com/tuxr/bible-cli/releases) and put `bible` on your
`PATH`.

## Commands

```
bible                         # TUI at last chapter, else Genesis 1 WEB
bible tui [ref]               # TUI starting at a reference
bible <ref>                   # print a verse, range, or chapter
bible read <ref>              # same as root lookup
bible search <query>          # full-text search
bible translations            # list translations
bible books [--testament OT|NT|AP]
bible random [--book PSA] [--testament NT]
bible version
bible completion …            # shell completion (no network)
```

References use the same strings the API already accepts (`John 3:16`,
`Romans 8:28-39`, `Psalm 23`). Book-only lookups (`bible John`) are rejected;
pass a chapter. `bible tui John` opens chapter 1.

Reserved names (`tui`, `read`, `search`, `translations`, `books`, `random`,
`version`, `help`, `completion`, `config`) are never treated as scripture.

## TUI

Keyboard-only chapter reader with a masthead, status line, and footer keymap.

| Key | Action |
| --- | --- |
| `j` / `↓` | next verse |
| `k` / `↑` | previous verse |
| `n` / `→` | next chapter |
| `p` / `←` | previous chapter |
| `b` | book picker (OT / NT / AP, filter as you type) |
| `t` | translation picker |
| `/` | search; Enter opens that chapter on the hit verse |
| `?` | help overlay |
| `esc` | back one view |
| `q` | quit |

A failed translation switch keeps the current chapter and translation.

## Flags and environment

| Flag | Env | Default |
| --- | --- | --- |
| `--translation`, `-t` | `BIBLE_TRANSLATION` | config, else `web` |
| `--api-url` | `BIBLE_API_URL` | `https://bible-api.dws-cloud.com` |
| `--theme` | `BIBLE_THEME` | config, else `dark` |
| `--json` | | off; on automatically for `search`, `translations`, and `books` when stdout is not a TTY |
| `--color` | `NO_COLOR` / `FORCE_COLOR` | `auto` (`never` if not a TTY or `NO_COLOR` is set) |
| `--red-letter` | `BIBLE_RED_LETTER` | config, else `true` |

Precedence is flag > env > config file > default.

`--color` accepts `auto`, `always`, or `never`. An explicit `--color` wins.
Otherwise non-empty `NO_COLOR` selects `never`; otherwise non-empty
`FORCE_COLOR` selects `always`. Invalid `--color` or `BIBLE_RED_LETTER` exits 1
before any request.

## Output

TTY lookup prints a header (`John 3:16 (WEB)`) and numbered verse lines. Words
of Jesus are crimson when red-letter and color are on.

Piped lookup without `--json` is UTF-8 with no ANSI: first line is the
canonical reference (no translation suffix); a single verse is the text only;
multiple verses are `verse-number<TAB>text`, one per line.

`--json` writes a stable envelope (reference, translation, verses with
`segments` when the API returned them). Lookup and `random` stay text when
piped unless `--json` is set. `search`, `translations`, and `books` emit JSON
when piped.

Exit codes: `0` ok · `1` usage/config · `2` not found (HTTP 400/404) · `3`
system (5xx, network, 429 after one retry).

## Themes and red-letter

Shipped themes: `dark` (default), `light`, and `auto` (terminal background
query; dark if unknown).

Verse and chapter fetches always request `segments=1`. TTY and TUI color
`speaker == "jesus"` with the theme’s crimson (`#C41E3A` on dark, `#9B1B30` on
light). Red-letter stays crimson across themes — contrast-tuned, never Accent
or another hue. `--red-letter=false` still fetches segments so JSON stays
complete, but paints everything as body text. Missing segments fall back to
plain `text`.

## Config

Path is `$XDG_CONFIG_HOME/bible/config.toml` on Linux
(`~/.config/bible/config.toml` by default) and
`~/Library/Application Support/bible/config.toml` on macOS
(`os.UserConfigDir`). A missing file is normal.

```toml
translation = "web"
theme = "dark"          # dark | light | auto
api_url = "https://bible-api.dws-cloud.com"
red_letter = true

[resume]
book = "JHN"
chapter = 3
```

The reader saves `[resume]` after a chapter loads successfully.

## Hebrew (WLC)

WLC text is printed as the API returns it. There is no bidirectional reordering
in v1, so Hebrew may not display RTL as expected in every terminal.

## License

MIT. Copyright (c) 2026 tuxr. See [LICENSE](LICENSE).

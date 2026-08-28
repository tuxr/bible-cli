package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tuxr/bible-cli/internal/api"
)

// SearchHits formats numbered TTY search results as "N  Reference  text".
func SearchHits(hits []api.SearchHit) string {
	var b strings.Builder
	for i, h := range hits {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("  ")
		b.WriteString(hitReference(h))
		b.WriteString("  ")
		b.WriteString(strings.TrimSpace(h.Text))
	}
	return b.String()
}

func hitReference(h api.SearchHit) string {
	if ref := strings.TrimSpace(h.Reference); ref != "" {
		return ref
	}
	name := strings.TrimSpace(h.BookName)
	if name == "" {
		name = strings.TrimSpace(h.Book)
	}
	return fmt.Sprintf("%s %d:%d", name, h.Chapter, h.Verse)
}

// Translations formats a TTY translation list as "id  name".
func Translations(list []api.Translation) string {
	var b strings.Builder
	for i, tr := range list {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(tr.ID)
		b.WriteString("  ")
		b.WriteString(tr.Name)
	}
	return b.String()
}

// Books formats a TTY book list as "id  name  chapters".
func Books(list []api.Book) string {
	var b strings.Builder
	for i, bk := range list {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(bk.ID)
		b.WriteString("  ")
		b.WriteString(bk.Name)
		b.WriteString("  ")
		b.WriteString(strconv.Itoa(bk.Chapters))
	}
	return b.String()
}

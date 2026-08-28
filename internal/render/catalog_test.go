package render

import (
	"strings"
	"testing"

	"github.com/tuxr/bible-cli/internal/api"
)

func TestSearchHitsNumbered(t *testing.T) {
	out := SearchHits([]api.SearchHit{
		{Reference: "Genesis 22:2", Text: "He said, “Now take your son, your only son, Isaac, whom you love, and go into the land of Moriah. Offer him there as a burnt offering on one of the mountains which I will tell you of.”"},
		{Reference: "Genesis 27:4", Text: "Make me savory food, such as I love, and bring it to me, that I may eat, and that my soul may bless you before I die.”"},
	})
	wantPrefix := "1  Genesis 22:2  He said"
	if !strings.Contains(out, wantPrefix) {
		t.Fatalf("missing %q in %q", wantPrefix, out)
	}
	if !strings.HasPrefix(out, "1  ") {
		t.Fatalf("want numbered first hit, got %q", out)
	}
	if !strings.Contains(out, "2  Genesis 27:4  Make me savory food") {
		t.Fatalf("missing second hit in %q", out)
	}
}

func TestTranslationsList(t *testing.T) {
	out := Translations([]api.Translation{
		{ID: "web", Name: "World English Bible"},
		{ID: "kjv", Name: "King James Version"},
	})
	if out != "web  World English Bible\nkjv  King James Version" {
		t.Fatalf("got %q", out)
	}
}

func TestBooksList(t *testing.T) {
	out := Books([]api.Book{
		{ID: "MAT", Name: "Matthew", Chapters: 28, Testament: "NT"},
	})
	if out != "MAT  Matthew  28" {
		t.Fatalf("got %q", out)
	}
}

package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUnmarshalVerseJohn316Segments(t *testing.T) {
	var got VerseResponse
	if err := json.Unmarshal(readFixture(t, "verse_john_3_16_segments.json"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Reference != "John 3:16" {
		t.Fatalf("reference = %q", got.Reference)
	}
	if len(got.Verses) != 1 {
		t.Fatalf("len(verses) = %d", len(got.Verses))
	}
	if len(got.Verses[0].Segments) == 0 {
		t.Fatal("expected segments")
	}
	if got.Verses[0].Segments[0].Speaker != "jesus" {
		t.Fatalf("speaker = %q, want jesus", got.Verses[0].Segments[0].Speaker)
	}
}

func TestUnmarshalChapterJohn3Segments(t *testing.T) {
	var got ChapterResponse
	if err := json.Unmarshal(readFixture(t, "chapter_john_3_segments.json"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Book.ID != "JHN" || got.Chapter != 3 {
		t.Fatalf("book/chapter = %s %d", got.Book.ID, got.Chapter)
	}
	if got.VerseCount != 36 {
		t.Fatalf("verse_count = %d", got.VerseCount)
	}

	var v3 *Verse
	for i := range got.Verses {
		if got.Verses[i].Verse == 3 {
			v3 = &got.Verses[i]
			break
		}
	}
	if v3 == nil {
		t.Fatal("missing John 3:3")
	}
	if len(v3.Segments) != 2 {
		t.Fatalf("John 3:3 segments = %d, want 2", len(v3.Segments))
	}
	if v3.Segments[0].Speaker != "narrator" {
		t.Fatalf("first speaker = %q, want narrator", v3.Segments[0].Speaker)
	}
	if v3.Segments[1].Speaker != "jesus" {
		t.Fatalf("second speaker = %q, want jesus", v3.Segments[1].Speaker)
	}
}

func TestUnmarshalSearchLove(t *testing.T) {
	var got SearchResponse
	if err := json.Unmarshal(readFixture(t, "search_love.json"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Query != "love" {
		t.Fatalf("query = %q", got.Query)
	}
	if got.Total < 2 {
		t.Fatalf("total = %d", got.Total)
	}
	if len(got.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(got.Results))
	}
	if got.Results[0].BookName == "" || got.Results[0].Reference == "" {
		t.Fatalf("first hit incomplete: %+v", got.Results[0])
	}
}

func TestUnmarshalTranslations(t *testing.T) {
	var got []Translation
	if err := json.Unmarshal(readFixture(t, "translations.json"), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("empty translations")
	}
	for _, tr := range got {
		if tr.ID == "" || tr.Name == "" {
			t.Fatalf("incomplete translation: %+v", tr)
		}
	}
}

func TestUnmarshalBooksNT(t *testing.T) {
	var got []Book
	if err := json.Unmarshal(readFixture(t, "books_nt.json"), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 27 {
		t.Fatalf("len(books) = %d, want 27", len(got))
	}
	if got[0].Testament != "NT" || got[0].ID == "" {
		t.Fatalf("first book: %+v", got[0])
	}
}

func TestUnmarshalErrorFixtures(t *testing.T) {
	cases := []struct {
		file string
		msg  string
	}{
		{"error_400.json", "Verse number must be at least 1"},
		{"error_404.json", "Not found"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			var got APIError
			if err := json.Unmarshal(readFixture(t, tc.file), &got); err != nil {
				t.Fatal(err)
			}
			if got.Msg != tc.msg {
				t.Fatalf("Msg = %q, want %q", got.Msg, tc.msg)
			}
		})
	}
}

func TestAPIErrorErrorReturnsMsg(t *testing.T) {
	e := &APIError{Status: 404, Kind: "not-found", Msg: "Not found"}
	if e.Error() != "Not found" {
		t.Fatalf("Error() = %q, want Msg", e.Error())
	}
}

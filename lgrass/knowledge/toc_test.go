package knowledge

import (
	"strings"
	"testing"
)

func TestFormatTOCEmpty(t *testing.T) {
	if got := FormatTOC(nil); got != "" {
		t.Errorf("FormatTOC(nil) = %q, want empty string", got)
	}
	if got := FormatTOC([]SearchResult{}); got != "" {
		t.Errorf("FormatTOC([]SearchResult{}) = %q, want empty string", got)
	}
}

func TestFormatTOCStandaloneEntries(t *testing.T) {
	results := []SearchResult{
		{ID: "standalone-note", Title: "Standalone Note", Tags: []string{"misc"}},
		{ID: "untagged-note", Title: "Untagged Note"},
	}
	got := FormatTOC(results)

	wantLine1 := "- standalone-note: Standalone Note [misc]"
	wantLine2 := "- untagged-note: Untagged Note"
	if !strings.Contains(got, wantLine1) {
		t.Errorf("FormatTOC output missing %q, got:\n%s", wantLine1, got)
	}
	if !strings.Contains(got, wantLine2) {
		t.Errorf("FormatTOC output missing %q, got:\n%s", wantLine2, got)
	}
}

func TestFormatTOCCollapsedBook(t *testing.T) {
	results := []SearchResult{
		{
			ID:           "auth-redesign",
			Title:        "Auth Redesign",
			Tags:         []string{"auth", "security"},
			IsBook:       true,
			ChapterCount: 2,
			Description:  "How auth got redesigned",
		},
	}
	got := FormatTOC(results)
	want := "- auth-redesign: Auth Redesign (book, 2 chapters) [auth, security] -- How auth got redesigned"
	if !strings.Contains(got, want) {
		t.Errorf("FormatTOC output missing %q, got:\n%s", want, got)
	}

	// Never leaks a chapter body, only the pointer fields.
	if strings.Contains(got, "the old system used sessions") {
		t.Error("FormatTOC leaked chapter body content into the TOC")
	}
}

func TestFormatTOCSingleChapterBookUsesSingular(t *testing.T) {
	results := []SearchResult{
		{ID: "solo-book", Title: "Solo Book", IsBook: true, ChapterCount: 1},
	}
	got := FormatTOC(results)
	if !strings.Contains(got, "(book, 1 chapter)") {
		t.Errorf("FormatTOC output = %q, want singular \"1 chapter\"", got)
	}
}

package knowledge

import (
	"fmt"
	"strings"
)

// FormatTOC renders search results as a pointers-only table of contents,
// meant for system-prompt injection rather than terminal display: titles,
// tags, and (for a book) its description and chapter count -- never a
// full body. Returns "" for no results, so a caller can skip injecting
// nothing.
func FormatTOC(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Project knowledge (query with `lgrass knowledge search \"...\"`, read an entry with `lgrass knowledge read <id>`):\n")
	for _, r := range results {
		fmt.Fprintf(&b, "- %s: %s", r.ID, r.Title)
		if r.IsBook {
			chapterWord := "chapters"
			if r.ChapterCount == 1 {
				chapterWord = "chapter"
			}
			fmt.Fprintf(&b, " (book, %d %s)", r.ChapterCount, chapterWord)
		}
		if len(r.Tags) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(r.Tags, ", "))
		}
		if r.IsBook && r.Description != "" {
			fmt.Fprintf(&b, " -- %s", r.Description)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

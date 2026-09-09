package knowledge

import (
	"fmt"
	"strings"
)

// Meant for system-prompt injection, so this never includes a full body, only pointers.
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

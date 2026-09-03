// Package knowledge stores and indexes knowledge entries under
// ~/.lemongrass/. Entries are plain markdown files with YAML frontmatter;
// the SQLite index in store.go is a derived, regenerable layer for tags
// and full-text search.
package knowledge

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Entry is one knowledge entry: frontmatter metadata plus a markdown body.
type Entry struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Tags      []string `yaml:"tags,omitempty"`
	Series    string   `yaml:"series,omitempty"`
	Part      int      `yaml:"part,omitempty"`
	PartTotal int      `yaml:"part_total,omitempty"`
	CreatedAt string   `yaml:"created_at"`
	UpdatedAt string   `yaml:"updated_at"`
	Body      string   `yaml:"-"`
}

const frontmatterDelim = "---"

// Marshal renders an entry in its on-disk file format: YAML frontmatter
// between --- delimiters, then the markdown body.
func (e Entry) Marshal() ([]byte, error) {
	meta, err := yaml.Marshal(e)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(frontmatterDelim + "\n")
	b.Write(meta)
	b.WriteString(frontmatterDelim + "\n\n")
	b.WriteString(strings.TrimRight(e.Body, "\n"))
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// ParseEntry reads an entry back from its on-disk format.
func ParseEntry(data []byte) (Entry, error) {
	text := string(data)
	if !strings.HasPrefix(text, frontmatterDelim) {
		return Entry{}, fmt.Errorf("missing frontmatter delimiter")
	}
	rest := text[len(frontmatterDelim):]
	end := strings.Index(rest, "\n"+frontmatterDelim)
	if end == -1 {
		return Entry{}, fmt.Errorf("unterminated frontmatter")
	}
	metaText := strings.TrimPrefix(rest[:end], "\n")
	body := rest[end+len("\n"+frontmatterDelim):]
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\n")

	var e Entry
	if err := yaml.Unmarshal([]byte(metaText), &e); err != nil {
		return Entry{}, fmt.Errorf("parsing frontmatter: %w", err)
	}
	e.Body = strings.TrimRight(body, "\n")
	return e, nil
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a title into a kebab-case id. Writing to an existing slug
// overwrites that entry.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugInvalid.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Now returns the current time formatted for frontmatter timestamps.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

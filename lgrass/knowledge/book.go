package knowledge

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Book groups a set of entries (its chapters) under one title. A book is
// its own file on disk, frontmatter only -- no body of its own.
type Book struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Tags        []string `yaml:"tags,omitempty"`
	Description string   `yaml:"description,omitempty"`
	CreatedAt   string   `yaml:"created_at"`
	UpdatedAt   string   `yaml:"updated_at"`
}

// Marshal renders a book in its on-disk file format: YAML frontmatter
// between --- delimiters, no body.
func (b Book) Marshal() ([]byte, error) {
	meta, err := yaml.Marshal(b)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	out.WriteString(frontmatterDelim + "\n")
	out.Write(meta)
	out.WriteString(frontmatterDelim + "\n")
	return []byte(out.String()), nil
}

// ParseBook reads a book back from its on-disk format.
func ParseBook(data []byte) (Book, error) {
	text := string(data)
	if !strings.HasPrefix(text, frontmatterDelim) {
		return Book{}, fmt.Errorf("missing frontmatter delimiter")
	}
	rest := text[len(frontmatterDelim):]
	end := strings.Index(rest, "\n"+frontmatterDelim)
	if end == -1 {
		return Book{}, fmt.Errorf("unterminated frontmatter")
	}
	metaText := strings.TrimPrefix(rest[:end], "\n")

	var b Book
	if err := yaml.Unmarshal([]byte(metaText), &b); err != nil {
		return Book{}, fmt.Errorf("parsing frontmatter: %w", err)
	}
	return b, nil
}

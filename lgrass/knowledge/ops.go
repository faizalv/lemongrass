package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOptions are the caller-supplied fields for a new or updated entry.
type WriteOptions struct {
	Title     string
	Tags      []string
	Series    string
	Part      int
	PartTotal int
	Body      string
}

// Write creates a new entry, or overwrites an existing one addressed by
// the same slug, on disk, then re-indexes it.
func Write(projectID string, opts WriteOptions) (Entry, error) {
	if strings.TrimSpace(opts.Body) == "" {
		return Entry{}, fmt.Errorf("empty body")
	}
	title := opts.Title
	if title == "" {
		title = firstLine(opts.Body)
	}
	if title == "" {
		return Entry{}, fmt.Errorf("no title given and body has no first line to derive one from")
	}

	id := Slugify(title)
	if id == "" {
		return Entry{}, fmt.Errorf("title %q produced an empty slug", title)
	}

	now := Now()
	createdAt := now
	if existing, err := Read(projectID, id); err == nil {
		createdAt = existing.CreatedAt
	}

	entry := Entry{
		ID:        id,
		Title:     title,
		Tags:      opts.Tags,
		Series:    opts.Series,
		Part:      opts.Part,
		PartTotal: opts.PartTotal,
		CreatedAt: createdAt,
		UpdatedAt: now,
		Body:      opts.Body,
	}

	if err := os.MkdirAll(Dir(projectID), 0o755); err != nil {
		return Entry{}, err
	}
	data, err := entry.Marshal()
	if err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(EntryPath(projectID, id), data, 0o644); err != nil {
		return Entry{}, err
	}

	store, err := Open(DBPath(), projectID)
	if err != nil {
		return Entry{}, err
	}
	defer store.Close()
	if err := store.Index(entry); err != nil {
		return Entry{}, err
	}

	return entry, nil
}

// Read loads one entry's full body from disk. The index is not consulted
// for content.
func Read(projectID, id string) (Entry, error) {
	data, err := os.ReadFile(EntryPath(projectID, id))
	if err != nil {
		return Entry{}, err
	}
	return ParseEntry(data)
}

func firstLine(body string) string {
	line := strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]
	return strings.TrimSpace(strings.TrimPrefix(line, "#"))
}

// Reindex rebuilds the index for one project from its entry files on disk.
func Reindex(projectID string) (int, error) {
	dir := Dir(projectID)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	store, err := Open(DBPath(), projectID)
	if err != nil {
		return 0, err
	}
	defer store.Close()

	count := 0
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return count, err
		}
		entry, err := ParseEntry(data)
		if err != nil {
			return count, fmt.Errorf("parsing %s: %w", f.Name(), err)
		}
		if err := store.Index(entry); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

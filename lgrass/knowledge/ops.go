package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOptions are the caller-supplied fields for a new or updated entry.
type WriteOptions struct {
	Title   string
	Tags    []string
	BookID  string
	Chapter int
	Body    string
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

	if opts.BookID != "" {
		if _, err := ReadBook(projectID, opts.BookID); err != nil {
			return Entry{}, fmt.Errorf("book %q not found, create it first with `lgrass knowledge book create`", opts.BookID)
		}
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
		BookID:    opts.BookID,
		Chapter:   opts.Chapter,
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

// BookOptions are the caller-supplied fields for a new or updated book.
type BookOptions struct {
	Title       string
	Tags        []string
	Description string
}

// CreateBook creates a new book, or overwrites an existing one addressed
// by the same slug, on disk, then re-indexes it.
func CreateBook(projectID string, opts BookOptions) (Book, error) {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		return Book{}, fmt.Errorf("title required")
	}

	id := Slugify(title)
	if id == "" {
		return Book{}, fmt.Errorf("title %q produced an empty slug", title)
	}

	now := Now()
	createdAt := now
	if existing, err := ReadBook(projectID, id); err == nil {
		createdAt = existing.CreatedAt
	}

	book := Book{
		ID:          id,
		Title:       title,
		Tags:        opts.Tags,
		Description: opts.Description,
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}

	if err := os.MkdirAll(BooksDir(projectID), 0o755); err != nil {
		return Book{}, err
	}
	data, err := book.Marshal()
	if err != nil {
		return Book{}, err
	}
	if err := os.WriteFile(BookPath(projectID, id), data, 0o644); err != nil {
		return Book{}, err
	}

	store, err := Open(DBPath(), projectID)
	if err != nil {
		return Book{}, err
	}
	defer store.Close()
	if err := store.IndexBook(book); err != nil {
		return Book{}, err
	}

	return book, nil
}

// ReadBook loads one book's metadata from disk.
func ReadBook(projectID, id string) (Book, error) {
	data, err := os.ReadFile(BookPath(projectID, id))
	if err != nil {
		return Book{}, err
	}
	return ParseBook(data)
}

func firstLine(body string) string {
	line := strings.SplitN(strings.TrimSpace(body), "\n", 2)[0]
	return strings.TrimSpace(strings.TrimPrefix(line, "#"))
}

// Reindex rebuilds the index for one project from its book and entry
// files on disk. Returns the number of entries and books indexed.
func Reindex(projectID string) (entries int, books int, err error) {
	store, err := Open(DBPath(), projectID)
	if err != nil {
		return 0, 0, err
	}
	defer store.Close()

	bookFiles, err := os.ReadDir(BooksDir(projectID))
	if err != nil && !os.IsNotExist(err) {
		return 0, 0, err
	}
	for _, f := range bookFiles {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(BooksDir(projectID), f.Name()))
		if err != nil {
			return 0, books, err
		}
		book, err := ParseBook(data)
		if err != nil {
			return 0, books, fmt.Errorf("parsing book %s: %w", f.Name(), err)
		}
		if err := store.IndexBook(book); err != nil {
			return 0, books, err
		}
		books++
	}

	dir := Dir(projectID)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, books, nil
	}
	if err != nil {
		return 0, books, err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return entries, books, err
		}
		entry, err := ParseEntry(data)
		if err != nil {
			return entries, books, fmt.Errorf("parsing %s: %w", f.Name(), err)
		}
		if err := store.Index(entry); err != nil {
			return entries, books, err
		}
		entries++
	}
	return entries, books, nil
}

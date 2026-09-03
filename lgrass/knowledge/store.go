package knowledge

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store is the derived, regenerable index over one project's knowledge
// entries and books -- tags and full-text search live here; the entries
// and books themselves stay plain markdown files on disk. Deleting this
// database and running Reindex rebuilds it from scratch.
type Store struct {
	db        *sql.DB
	projectID string
}

const schema = `
CREATE TABLE IF NOT EXISTS entries (
	project_id TEXT NOT NULL,
	id TEXT NOT NULL,
	title TEXT NOT NULL,
	book_id TEXT,
	chapter INTEGER,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_entries_book ON entries(project_id, book_id, chapter);
CREATE TABLE IF NOT EXISTS tags (
	project_id TEXT NOT NULL,
	entry_id TEXT NOT NULL,
	tag TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tags_lookup ON tags(project_id, tag);
CREATE INDEX IF NOT EXISTS idx_tags_entry ON tags(project_id, entry_id);
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
	project_id UNINDEXED,
	entry_id UNINDEXED,
	title,
	body,
	tags
);
CREATE TABLE IF NOT EXISTS books (
	project_id TEXT NOT NULL,
	id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, id)
);
CREATE TABLE IF NOT EXISTS book_tags (
	project_id TEXT NOT NULL,
	book_id TEXT NOT NULL,
	tag TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_book_tags_lookup ON book_tags(project_id, tag);
CREATE INDEX IF NOT EXISTS idx_book_tags_book ON book_tags(project_id, book_id);
CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(
	project_id UNINDEXED,
	book_id UNINDEXED,
	title,
	description,
	tags
);
`

// Open opens (creating and migrating if needed) the shared index database
// at dbPath, scoped to one project's entries and books.
func Open(dbPath, projectID string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return &Store{db: db, projectID: projectID}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Index upserts one entry's metadata, tags, and searchable text.
func (s *Store) Index(e Entry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO entries (project_id, id, title, book_id, chapter, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project_id, id) DO UPDATE SET
			title = excluded.title,
			book_id = excluded.book_id,
			chapter = excluded.chapter,
			updated_at = excluded.updated_at
	`, s.projectID, e.ID, e.Title, nullable(e.BookID), e.Chapter, e.CreatedAt, e.UpdatedAt); err != nil {
		return fmt.Errorf("indexing entry: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM tags WHERE project_id = ? AND entry_id = ?`, s.projectID, e.ID); err != nil {
		return err
	}
	for _, tag := range e.Tags {
		if _, err := tx.Exec(`INSERT INTO tags (project_id, entry_id, tag) VALUES (?, ?, ?)`, s.projectID, e.ID, tag); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM entries_fts WHERE project_id = ? AND entry_id = ?`, s.projectID, e.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO entries_fts (project_id, entry_id, title, body, tags)
		VALUES (?, ?, ?, ?, ?)
	`, s.projectID, e.ID, e.Title, e.Body, strings.Join(e.Tags, " ")); err != nil {
		return err
	}

	return tx.Commit()
}

// IndexBook upserts one book's metadata, tags, and searchable text.
func (s *Store) IndexBook(b Book) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO books (project_id, id, title, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (project_id, id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, s.projectID, b.ID, b.Title, nullable(b.Description), b.CreatedAt, b.UpdatedAt); err != nil {
		return fmt.Errorf("indexing book: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM book_tags WHERE project_id = ? AND book_id = ?`, s.projectID, b.ID); err != nil {
		return err
	}
	for _, tag := range b.Tags {
		if _, err := tx.Exec(`INSERT INTO book_tags (project_id, book_id, tag) VALUES (?, ?, ?)`, s.projectID, b.ID, tag); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM books_fts WHERE project_id = ? AND book_id = ?`, s.projectID, b.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO books_fts (project_id, book_id, title, description, tags)
		VALUES (?, ?, ?, ?, ?)
	`, s.projectID, b.ID, b.Title, b.Description, strings.Join(b.Tags, " ")); err != nil {
		return err
	}

	return tx.Commit()
}

// SearchResult holds a search hit's metadata -- either a standalone entry,
// an entry belonging to a book, or (when IsBook is true) a book collapsed
// to a single line representing all of its chapters.
type SearchResult struct {
	ID           string
	Title        string
	Tags         []string
	BookID       string
	Chapter      int
	IsBook       bool
	ChapterCount int
}

func (s *Store) scanEntryRows(rows *sql.Rows) ([]SearchResult, error) {
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var bookID sql.NullString
		var chapter sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Title, &bookID, &chapter); err != nil {
			return nil, err
		}
		r.BookID = bookID.String
		r.Chapter = int(chapter.Int64)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range results {
		tags, err := s.tagsFor(results[i].ID)
		if err != nil {
			return nil, err
		}
		results[i].Tags = tags
	}
	return results, nil
}

func (s *Store) tagsFor(entryID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT tag FROM tags WHERE project_id = ? AND entry_id = ? ORDER BY tag`, s.projectID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *Store) bookTagsFor(bookID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT tag FROM book_tags WHERE project_id = ? AND book_id = ? ORDER BY tag`, s.projectID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// bookSummary builds the collapsed one-line SearchResult for a book: its
// own title/tags plus how many chapters it has.
func (s *Store) bookSummary(bookID string) (SearchResult, error) {
	var title string
	var description sql.NullString
	err := s.db.QueryRow(`SELECT title, description FROM books WHERE project_id = ? AND id = ?`, s.projectID, bookID).Scan(&title, &description)
	if err != nil {
		return SearchResult{}, fmt.Errorf("looking up book %s: %w", bookID, err)
	}
	tags, err := s.bookTagsFor(bookID)
	if err != nil {
		return SearchResult{}, err
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM entries WHERE project_id = ? AND book_id = ?`, s.projectID, bookID).Scan(&count); err != nil {
		return SearchResult{}, err
	}
	return SearchResult{ID: bookID, Title: title, Tags: tags, IsBook: true, ChapterCount: count}, nil
}

func (s *Store) searchEntries(query string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		rows, err := s.db.Query(`
			SELECT id, title, book_id, chapter FROM entries
			WHERE project_id = ?
			ORDER BY updated_at DESC
		`, s.projectID)
		if err != nil {
			return nil, err
		}
		return s.scanEntryRows(rows)
	}

	rows, err := s.db.Query(`
		SELECT e.id, e.title, e.book_id, e.chapter
		FROM entries_fts f
		JOIN entries e ON e.project_id = f.project_id AND e.id = f.entry_id
		WHERE f.project_id = ? AND entries_fts MATCH ?
		ORDER BY rank
	`, s.projectID, query)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	return s.scanEntryRows(rows)
}

func (s *Store) searchBooks(query string) ([]SearchResult, error) {
	var ids []string
	if strings.TrimSpace(query) == "" {
		rows, err := s.db.Query(`SELECT id FROM books WHERE project_id = ? ORDER BY updated_at DESC`, s.projectID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		rows, err := s.db.Query(`
			SELECT book_id FROM books_fts
			WHERE project_id = ? AND books_fts MATCH ?
			ORDER BY rank
		`, s.projectID, query)
		if err != nil {
			return nil, fmt.Errorf("book search query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	var results []SearchResult
	for _, id := range ids {
		r, err := s.bookSummary(id)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// Search returns entries and books matching a full-text query, or
// everything ordered by most recently updated when query is empty. Any
// entry belonging to a book collapses into a single line for that book
// (title, tags, chapter count) rather than one line per chapter; a book
// whose own title/description matches but whose chapters didn't is
// included the same way.
func (s *Store) Search(query string) ([]SearchResult, error) {
	entryResults, err := s.searchEntries(query)
	if err != nil {
		return nil, err
	}

	var out []SearchResult
	seenBooks := make(map[string]bool)
	for _, r := range entryResults {
		if r.BookID == "" {
			out = append(out, r)
			continue
		}
		if seenBooks[r.BookID] {
			continue
		}
		seenBooks[r.BookID] = true
		book, err := s.bookSummary(r.BookID)
		if err != nil {
			return nil, err
		}
		out = append(out, book)
	}

	bookHits, err := s.searchBooks(query)
	if err != nil {
		return nil, err
	}
	for _, b := range bookHits {
		if seenBooks[b.ID] {
			continue
		}
		seenBooks[b.ID] = true
		out = append(out, b)
	}

	return out, nil
}

// Chapters returns every entry in a book, in chapter order.
func (s *Store) Chapters(bookID string) ([]SearchResult, error) {
	rows, err := s.db.Query(`
		SELECT id, title, book_id, chapter FROM entries
		WHERE project_id = ? AND book_id = ?
		ORDER BY chapter ASC
	`, s.projectID, bookID)
	if err != nil {
		return nil, err
	}
	return s.scanEntryRows(rows)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

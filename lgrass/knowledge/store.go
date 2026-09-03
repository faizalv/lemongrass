package knowledge

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store is the derived, regenerable index over one project's knowledge
// entries -- tags and full-text search live here; the entries themselves
// stay plain markdown files on disk. Deleting this database and running
// Reindex rebuilds it from scratch.
type Store struct {
	db        *sql.DB
	projectID string
}

const schema = `
CREATE TABLE IF NOT EXISTS entries (
	project_id TEXT NOT NULL,
	id TEXT NOT NULL,
	title TEXT NOT NULL,
	series TEXT,
	part INTEGER,
	part_total INTEGER,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, id)
);
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
`

// Open opens (creating and migrating if needed) the shared index database
// at dbPath, scoped to one project's entries.
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
		INSERT INTO entries (project_id, id, title, series, part, part_total, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project_id, id) DO UPDATE SET
			title = excluded.title,
			series = excluded.series,
			part = excluded.part,
			part_total = excluded.part_total,
			updated_at = excluded.updated_at
	`, s.projectID, e.ID, e.Title, nullable(e.Series), e.Part, e.PartTotal, e.CreatedAt, e.UpdatedAt); err != nil {
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

// SearchResult holds a search hit's metadata: title, tags, series -- not
// the entry body.
type SearchResult struct {
	ID        string
	Title     string
	Tags      []string
	Series    string
	Part      int
	PartTotal int
}

func (s *Store) scanResults(rows *sql.Rows) ([]SearchResult, error) {
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var series sql.NullString
		var part, partTotal sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Title, &series, &part, &partTotal); err != nil {
			return nil, err
		}
		r.Series = series.String
		r.Part = int(part.Int64)
		r.PartTotal = int(partTotal.Int64)
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

// Search returns entries matching a full-text query, or every entry
// ordered by most recently updated when query is empty.
func (s *Store) Search(query string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		rows, err := s.db.Query(`
			SELECT id, title, series, part, part_total FROM entries
			WHERE project_id = ?
			ORDER BY updated_at DESC
		`, s.projectID)
		if err != nil {
			return nil, err
		}
		return s.scanResults(rows)
	}

	rows, err := s.db.Query(`
		SELECT e.id, e.title, e.series, e.part, e.part_total
		FROM entries_fts f
		JOIN entries e ON e.project_id = f.project_id AND e.id = f.entry_id
		WHERE f.project_id = ? AND entries_fts MATCH ?
		ORDER BY rank
	`, s.projectID, query)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	return s.scanResults(rows)
}

// Series returns every entry in a series, in part order.
func (s *Store) Series(seriesID string) ([]SearchResult, error) {
	rows, err := s.db.Query(`
		SELECT id, title, series, part, part_total FROM entries
		WHERE project_id = ? AND series = ?
		ORDER BY part ASC
	`, s.projectID, seriesID)
	if err != nil {
		return nil, err
	}
	return s.scanResults(rows)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

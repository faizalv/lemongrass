package knowledge

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

// Dir returns the directory holding one project's knowledge entry files.
func Dir(projectID string) string {
	return filepath.Join(config.Dir(), "knowledge", projectID)
}

// DBPath returns the shared, cross-project index database path.
func DBPath() string {
	return filepath.Join(config.Dir(), "knowledge.db")
}

// EntryPath returns the on-disk path for one entry's file.
func EntryPath(projectID, id string) string {
	return filepath.Join(Dir(projectID), id+".md")
}

// BooksDir returns the directory holding one project's book files.
func BooksDir(projectID string) string {
	return filepath.Join(Dir(projectID), "books")
}

// BookPath returns the on-disk path for one book's file.
func BookPath(projectID, id string) string {
	return filepath.Join(BooksDir(projectID), id+".md")
}

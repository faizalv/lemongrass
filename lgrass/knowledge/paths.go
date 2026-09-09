package knowledge

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func Dir(projectID string) string {
	return filepath.Join(config.Dir(), "knowledge", projectID)
}

func DBPath() string {
	return filepath.Join(config.Dir(), "knowledge.db")
}

func EntryPath(projectID, id string) string {
	return filepath.Join(Dir(projectID), id+".md")
}

func BooksDir(projectID string) string {
	return filepath.Join(Dir(projectID), "books")
}

func BookPath(projectID, id string) string {
	return filepath.Join(BooksDir(projectID), id+".md")
}

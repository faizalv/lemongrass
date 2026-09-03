// Package project resolves a directory to its registered lemongrass project.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

// Project mirrors ui/src/main/projects.ts's shape in
// ~/.lemongrass/projects.json.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func registryPath() string {
	return filepath.Join(config.Dir(), "projects.json")
}

func loadAll() ([]Project, error) {
	data, err := os.ReadFile(registryPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", registryPath(), err)
	}
	return projects, nil
}

// Resolve matches a directory to a registered project: exact path first,
// then the nearest registered ancestor.
func Resolve(dir string) (Project, error) {
	projects, err := loadAll()
	if err != nil {
		return Project{}, err
	}
	if len(projects) == 0 {
		return Project{}, fmt.Errorf("no projects registered in %s", registryPath())
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return Project{}, err
	}
	abs = filepath.Clean(abs)

	for {
		for _, p := range projects {
			if filepath.Clean(p.Path) == abs {
				return p, nil
			}
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}

	return Project{}, fmt.Errorf("%s is not inside a registered lemongrass project", dir)
}

// ResolveCwd resolves the current working directory to its project.
func ResolveCwd() (Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Project{}, err
	}
	return Resolve(cwd)
}

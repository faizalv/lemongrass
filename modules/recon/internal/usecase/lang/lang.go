package lang

import "github.com/faizalv/lemongrass/modules/recon/entity"

// Parser knows how to detect and parse a specific language's project structure.
// Priority controls evaluation order: higher runs first.
type Parser interface {
	Name() string
	Priority() int
	Detect(dir string) bool
	Parse(dir string) (*entity.ProjectTree, error)
}

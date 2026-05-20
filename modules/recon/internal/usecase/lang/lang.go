package lang

import "github.com/faizalv/lemongrass/modules/recon/entity"

// Parser knows how to detect and parse a specific language's project structure
// into a ProjectTree. Adding a new language = implement this interface and
// register it in the engine.
type Parser interface {
	Name() string
	Detect(dir string) bool
	Parse(dir string) (*entity.ProjectTree, error)
}

package lang

import "github.com/faizalv/lemongrass/modules/recon/entity"

type Ignorer interface {
	Match(relPath string) bool
	Patterns() []string
}

// Parser knows how to detect and parse a specific language's project structure.
// Priority controls evaluation order: higher runs first.
// All parsers return ParseResult. In-process parsers convert from their internal
// representation; ContainerParser deserialises from the lg-lang HTTP response.
type Parser interface {
	Name() string
	Priority() int
	Detect(dir string) bool
	Parse(dir string, ig Ignorer) (*entity.ParseResult, error)
	ParseFiles(dir string, ig Ignorer, paths []string) (*entity.ParseResult, error)
}

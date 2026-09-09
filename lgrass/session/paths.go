// Package session needs no PTY access and no Electron dependency, the same domain split as lgrass/knowledge.
package session

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func DBPath() string {
	return filepath.Join(config.Dir(), "sessions.db")
}

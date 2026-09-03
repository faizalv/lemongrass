// Package session tracks per-project session liveness and recent file
// activity, fed entirely by Claude Code's own hook events. It needs no
// PTY access and no Electron dependency, the same domain split as
// lgrass/knowledge.
package session

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

// DBPath returns the shared, cross-project session database path.
func DBPath() string {
	return filepath.Join(config.Dir(), "sessions.db")
}

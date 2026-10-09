package agent

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/internal/config"
)

func SocketPath() string {
	return filepath.Join(config.Dir(), "agent.sock")
}

package gatekeeper

import (
	"path/filepath"

	"github.com/faizalv/lemongrass/config"
)

func SocketPath() string {
	return filepath.Join(config.Dir(), "vault.sock")
}

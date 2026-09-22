//go:build !linux && !darwin

package main

import "fmt"

func installService(home string) error {
	_ = home
	return fmt.Errorf("lgrassconf install is only supported on Linux (systemd) and macOS (LaunchAgent)")
}

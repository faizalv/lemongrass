package main

import (
	"os"
	"path/filepath"
)

const (
	unitName          = "lgrassconf.service"
	legacyUnitName    = "lgconf.service"
	launchAgentLabel  = "com.lemongrass.lgrassconf"
	legacyLaunchLabel = "com.lemongrass.lgconf"
)

func installUnit(home string) error {
	return installService(home)
}

func resolveBinary() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	return binary, nil
}

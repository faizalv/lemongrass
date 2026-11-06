// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// localTarget is a download's local file. Without replace it is the target itself, created only
// if the name is free; with replace it is a temporary file beside the target, renamed over it on
// success, so a failed or cancelled download never destroys the file it was meant to replace.
type localTarget struct {
	f      *os.File
	commit func() error
	abort  func()
}

// checkLocalTarget is the preflight half of openLocalTarget, run before any remote work starts.
func checkLocalTarget(local string, replace bool) error {
	fi, err := os.Lstat(local)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("sshsession: download to %s: %w", local, err)
	case fi.IsDir():
		return withCode(CodeIsDir, "sshsession: %s is a folder", local)
	case !replace:
		return withCode(CodeExists, "sshsession: %s already exists", local)
	}
	return nil
}

func openLocalTarget(local string, replace bool) (*localTarget, error) {
	if !replace {
		f, err := os.OpenFile(local, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return nil, withCode(CodeExists, "sshsession: %s already exists", local)
			}
			return nil, fmt.Errorf("sshsession: download to %s: %w", local, err)
		}
		return &localTarget{
			f:      f,
			commit: f.Close,
			abort:  func() { f.Close(); _ = os.Remove(local) },
		}, nil
	}
	f, err := os.CreateTemp(filepath.Dir(local), "."+filepath.Base(local)+".lgrass-*")
	if err != nil {
		return nil, fmt.Errorf("sshsession: download to %s: %w", local, err)
	}
	tmp := f.Name()
	// CreateTemp makes the file 0600; a download gets the usual 0644, as without replace.
	_ = f.Chmod(0o644)
	return &localTarget{
		f: f,
		commit: func() error {
			if err := f.Close(); err != nil {
				return err
			}
			return os.Rename(tmp, local)
		},
		abort: func() { f.Close(); _ = os.Remove(tmp) },
	}, nil
}

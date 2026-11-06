// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/pkg/sftp"
)

// maxEditBytes is the largest file the remote editor opens.
const maxEditBytes = 2 << 20

// errInPlace tells writeText to write the target in place, because a temp file and rename cannot
// keep the file's identity: the folder is not writable, the owner cannot be kept, or the server
// cannot rename over an existing file.
var errInPlace = errors.New("write in place")

func readText(c *sftp.Client, p string) (any, error) {
	fi, err := c.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("sshsession: open %s: %w", p, err)
	}
	if fi.IsDir() {
		return nil, withCode(CodeIsDir, "sshsession: %s is a folder", p)
	}
	if fi.Size() > maxEditBytes {
		return nil, withCode(CodeTooLarge, "sshsession: %s is larger than 2 MB", p)
	}
	f, err := c.Open(p)
	if err != nil {
		return nil, fmt.Errorf("sshsession: open %s: %w", p, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEditBytes+1))
	if err != nil {
		return nil, fmt.Errorf("sshsession: read %s: %w", p, err)
	}
	if len(data) > maxEditBytes {
		return nil, withCode(CodeTooLarge, "sshsession: %s is larger than 2 MB", p)
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, withCode(CodeBinary, "sshsession: %s is not a text file", p)
	}
	return map[string]any{"content": string(data), "mtime": fi.ModTime().UnixMilli(), "size": len(data)}, nil
}

// sameSecond compares a remote mtime with one the drawer sent back. SFTP v3 carries mtime in whole
// seconds, so the drawer's milliseconds are always a multiple of 1000 and only seconds compare.
func sameSecond(fi os.FileInfo, mtimeMillis int64) bool {
	return fi.ModTime().Unix() == mtimeMillis/1000
}

// writeText saves an edited file. mtime is the one read returned; 0 means a new file.
func writeText(c *sftp.Client, p, content string, mtime int64, force bool) (any, error) {
	lfi, err := c.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		if mtime != 0 && !force {
			return nil, withCode(CodeChanged, "sshsession: %s was deleted on the server", p)
		}
		if err := writeNew(c, p, content); err != nil {
			return nil, err
		}
		return statMtime(c, p)
	}
	if err != nil {
		return nil, fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	link := lfi.Mode()&fs.ModeSymlink != 0
	fi := lfi
	if link {
		if fi, err = c.Stat(p); err != nil {
			return nil, fmt.Errorf("sshsession: save %s: %w", p, err)
		}
	}
	if fi.IsDir() {
		return nil, withCode(CodeIsDir, "sshsession: %s is a folder", p)
	}
	if !force {
		if mtime == 0 {
			return nil, withCode(CodeExists, "sshsession: %s already exists", p)
		}
		if !sameSecond(fi, mtime) {
			return nil, withCode(CodeChanged, "sshsession: %s changed on the server since it was opened", p)
		}
	}
	// Renaming over a symlink would replace the link with a plain file, so a link is written
	// through in place.
	err = errInPlace
	if !link {
		err = replaceViaTemp(c, p, content, fi)
	}
	if errors.Is(err, errInPlace) {
		err = writeInPlace(c, p, content)
	}
	if err != nil {
		return nil, err
	}
	return statMtime(c, p)
}

func statMtime(c *sftp.Client, p string) (any, error) {
	fi, err := c.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("sshsession: saved %s but cannot read it back: %w", p, err)
	}
	return map[string]int64{"mtime": fi.ModTime().UnixMilli()}, nil
}

func writeContent(f *sftp.File, content string) error {
	_, err := f.ReadFromWithConcurrency(strings.NewReader(content), 0)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeNew(c *sftp.Client, p, content string) error {
	f, err := c.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	if err := writeContent(f, content); err != nil {
		return fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	return nil
}

func writeInPlace(c *sftp.Client, p, content string) error {
	f, err := c.OpenFile(p, os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	if err := writeContent(f, content); err != nil {
		return fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	return nil
}

// replaceViaTemp writes a temp file beside p with p's mode and owner and renames it over p, so a
// reader never sees a half-written file. It returns errInPlace when that cannot keep the file as
// it was, and removes the temp file on any failure.
func replaceViaTemp(c *sftp.Client, p, content string, fi os.FileInfo) error {
	if _, ok := c.HasExtension("posix-rename@openssh.com"); !ok {
		return errInPlace
	}
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	tmp := path.Join(path.Dir(p), "."+path.Base(p)+".lgrass-"+hex.EncodeToString(rnd[:]))
	f, err := c.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		if errors.Is(err, sftp.ErrSSHFxConnectionLost) {
			return fmt.Errorf("sshsession: save %s: %w", p, err)
		}
		return errInPlace
	}
	done := false
	defer func() {
		if !done {
			_ = c.Remove(tmp)
		}
	}()
	if err := writeContent(f, content); err != nil {
		return fmt.Errorf("sshsession: save %s: %w", p, err)
	}
	if err := c.Chmod(tmp, fi.Mode()&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky)); err != nil {
		return errInPlace
	}
	if want, ok := fi.Sys().(*sftp.FileStat); ok {
		tfi, err := c.Stat(tmp)
		if err != nil {
			return errInPlace
		}
		if got, ok := tfi.Sys().(*sftp.FileStat); !ok || got.UID != want.UID || got.GID != want.GID {
			if err := c.Chown(tmp, int(want.UID), int(want.GID)); err != nil {
				return errInPlace
			}
		}
	}
	if err := c.PosixRename(tmp, p); err != nil {
		return errInPlace
	}
	done = true
	return nil
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package sshsession

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/sftp"
)

// Entry is one row of the file browser.
type Entry struct {
	Name  string `json:"name"`
	Dir   bool   `json:"dir"`
	Link  bool   `json:"link"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
	Mtime int64  `json:"mtime"`
}

// resolve makes p absolute against the remote home, since SFTP paths are otherwise relative to
// wherever the server started.
func resolve(home, p string) string {
	if p == "" {
		return home
	}
	if !path.IsAbs(p) {
		p = path.Join(home, p)
	}
	return path.Clean(p)
}

// modeString renders a mode the way ls does, with "l" for a link where Go writes "L".
func modeString(m fs.FileMode) string {
	t := "-"
	switch {
	case m&fs.ModeDir != 0:
		t = "d"
	case m&fs.ModeSymlink != 0:
		t = "l"
	case m&fs.ModeNamedPipe != 0:
		t = "p"
	case m&fs.ModeSocket != 0:
		t = "s"
	case m&fs.ModeCharDevice != 0:
		t = "c"
	case m&fs.ModeDevice != 0:
		t = "b"
	}
	return t + m.Perm().String()[1:]
}

// lookups bounds concurrent stats of link targets in one listing.
const lookups = 8

func list(c *sftp.Client, dir string) (any, error) {
	infos, err := c.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("sshsession: list %s: %w", dir, err)
	}
	entries := make([]Entry, len(infos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, lookups)
	for i, fi := range infos {
		e := Entry{
			Name:  fi.Name(),
			Dir:   fi.IsDir(),
			Link:  fi.Mode()&fs.ModeSymlink != 0,
			Size:  fi.Size(),
			Mode:  modeString(fi.Mode()),
			Mtime: fi.ModTime().UnixMilli(),
		}
		entries[i] = e
		if e.Link {
			// A link takes its folder-ness and size from its target, so a link to a folder opens
			// on double-click; a broken link stays a plain row.
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, name string) {
				defer wg.Done()
				defer func() { <-sem }()
				if t, err := c.Stat(path.Join(dir, name)); err == nil {
					entries[i].Dir = t.IsDir()
					entries[i].Size = t.Size()
				}
			}(i, e.Name)
		}
	}
	wg.Wait()
	for i := range entries {
		if entries[i].Dir {
			entries[i].Size = 0
		}
	}
	sort.Slice(entries, func(a, b int) bool {
		if entries[a].Dir != entries[b].Dir {
			return entries[a].Dir
		}
		la, lb := strings.ToLower(entries[a].Name), strings.ToLower(entries[b].Name)
		if la != lb {
			return la < lb
		}
		return entries[a].Name < entries[b].Name
	})
	return map[string]any{"path": dir, "entries": entries}, nil
}

// taken reports whether a remote name is already used, links and broken links included.
func taken(c *sftp.Client, p string) (bool, error) {
	_, err := c.Lstat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func mkdir(c *sftp.Client, p string) error {
	if ok, err := taken(c, p); err != nil {
		return fmt.Errorf("sshsession: mkdir %s: %w", p, err)
	} else if ok {
		return withCode(CodeExists, "sshsession: %s already exists", p)
	}
	if err := c.Mkdir(p); err != nil {
		return fmt.Errorf("sshsession: mkdir %s: %w", p, err)
	}
	return nil
}

func rename(c *sftp.Client, from, to string) error {
	// SFTP rename refuses an existing target on OpenSSH but replaces it on some servers, so the
	// check is made here to keep "never overwrite" true everywhere.
	if ok, err := taken(c, to); err != nil {
		return fmt.Errorf("sshsession: rename %s: %w", from, err)
	} else if ok {
		return withCode(CodeExists, "sshsession: %s already exists", to)
	}
	if err := c.Rename(from, to); err != nil {
		return fmt.Errorf("sshsession: rename %s: %w", from, err)
	}
	return nil
}

func remove(c *sftp.Client, p string) error {
	if p == "/" {
		return errors.New("sshsession: refusing to remove /")
	}
	if err := removeAll(c, p); err != nil {
		return fmt.Errorf("sshsession: remove %s: %w", p, err)
	}
	return nil
}

// removeAll deletes p and, for a real folder, everything under it. It uses Lstat so a link to a
// folder loses only the link, never the target's contents; sftp's RemoveAll follows links.
func removeAll(c *sftp.Client, p string) error {
	fi, err := c.Lstat(p)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return c.Remove(p)
	}
	children, err := c.ReadDir(p)
	if err != nil {
		return err
	}
	for _, ch := range children {
		if err := removeAll(c, path.Join(p, ch.Name())); err != nil {
			return err
		}
	}
	return c.RemoveDirectory(p)
}

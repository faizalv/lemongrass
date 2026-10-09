package agentconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

var accountNamePattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

var sharedEntries = []string{"projects", "skills", "plugins", "plans", "file-history", "paste-cache"}

type Info struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

type accountsFile struct {
	Accounts []string `json:"accounts"`
}

// Accounts manages the Claude Code accounts that live beside the primary ~/.claude. HookBinary is the lgrassd path written into each account's hooks, and empty means no hooks are registered.
type Accounts struct {
	Home       string
	HookBinary string
}

func (a Accounts) FilePath() string {
	return filepath.Join(a.Home, ".lemongrass", "accounts.json")
}

func (a Accounts) PrimaryDir() string {
	return filepath.Join(a.Home, ".claude")
}

func (a Accounts) Dir(name string) string {
	return filepath.Join(a.Home, ".claude-"+name)
}

func (a Accounts) SettingsPath(name string) string {
	return filepath.Join(a.Dir(name), "settings.json")
}

func ValidName(name string) error {
	if name == "primary" || !accountNamePattern.MatchString(name) {
		return fmt.Errorf("agentconf: account name %q must be lowercase letters, digits and hyphens, up to 32 characters, and not primary", name)
	}
	return nil
}

func (a Accounts) Names() ([]string, error) {
	data, err := os.ReadFile(a.FilePath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var file accountsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("agentconf: parsing %s: %w", a.FilePath(), err)
	}
	return file.Accounts, nil
}

func (a Accounts) saveNames(names []string) error {
	if names == nil {
		names = []string{}
	}
	data, err := json.MarshalIndent(accountsFile{Accounts: names}, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(a.FilePath(), append(data, '\n'), 0o600)
}

func (a Accounts) List() ([]Info, error) {
	names, err := a.Names()
	if err != nil {
		return nil, err
	}
	accounts := make([]Info, 0, len(names))
	for _, name := range names {
		accounts = append(accounts, Info{Name: name, Dir: "~/.claude-" + name})
	}
	return accounts, nil
}

func (a Accounts) Add(name string) (Info, error) {
	if err := ValidName(name); err != nil {
		return Info{}, err
	}
	names, err := a.Names()
	if err != nil {
		return Info{}, err
	}
	for _, existing := range names {
		if existing == name {
			return Info{}, fmt.Errorf("agentconf: account %q already exists", name)
		}
	}
	dir := a.Dir(name)
	if _, err := os.Lstat(dir); err == nil {
		return Info{}, fmt.Errorf("agentconf: %s already exists and is not managed by lemongrass", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Info{}, err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return Info{}, err
	}
	if err := a.saveNames(append(names, name)); err != nil {
		os.Remove(dir)
		return Info{}, err
	}
	if err := a.ReconcileOne(name); err != nil {
		return Info{}, err
	}
	return Info{Name: name, Dir: "~/.claude-" + name}, nil
}

func (a Accounts) Remove(name string) error {
	names, err := a.Names()
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(names))
	found := false
	for _, existing := range names {
		if existing == name {
			found = true
			continue
		}
		kept = append(kept, existing)
	}
	if !found {
		return fmt.Errorf("agentconf: no account named %q", name)
	}
	dir := a.Dir(name)
	info, err := os.Lstat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("agentconf: %s is not a directory, leaving it in place", dir)
		}
		for _, entry := range sharedEntries {
			if err := removeIfSymlink(filepath.Join(dir, entry)); err != nil {
				return err
			}
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return a.saveNames(kept)
}

func removeIfSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return os.Remove(path)
}

func (a Accounts) Reconcile() error {
	names, err := a.Names()
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range names {
		if err := a.ReconcileOne(name); err != nil {
			errs = append(errs, fmt.Errorf("agentconf: account %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

func (a Accounts) ReconcileOne(name string) error {
	if err := os.MkdirAll(a.Dir(name), 0o700); err != nil {
		return err
	}
	var errs []error
	for _, entry := range sharedEntries {
		if err := a.linkShared(name, entry); err != nil {
			errs = append(errs, err)
		}
	}
	if a.HookBinary != "" {
		if err := SyncHookConfig(a.SettingsPath(name), a.HookBinary, ReconcileClaudeSettings); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a Accounts) linkShared(name, entry string) error {
	src := filepath.Join(a.PrimaryDir(), entry)
	if _, err := os.Lstat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	dst := filepath.Join(a.Dir(name), entry)
	info, err := os.Lstat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return os.Symlink(src, dst)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(dst)
		if err != nil {
			return err
		}
		if target != src {
			return fmt.Errorf("agentconf: %s points to %s, not replaced", dst, target)
		}
		return nil
	}
	if info.IsDir() {
		entries, err := os.ReadDir(dst)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dst); err != nil {
				return err
			}
			return os.Symlink(src, dst)
		}
	}
	return fmt.Errorf("agentconf: %s holds data of its own, not replaced", dst)
}

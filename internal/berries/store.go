// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/internal/atomicfile"
)

// Store is a project's local berries data: seeds, runs and stains. It lives under the lemongrass home, never in the
// repository, because seeds and evidence hold data from the environment they were recorded against.
type Store struct {
	Dir string
}

// StoreDir is the store of one project under lemongrass's home directory.
func StoreDir(lemongrassDir, projectID string) string {
	return filepath.Join(lemongrassDir, "berries", projectID)
}

// Header is one request header a seed was sent with. Only the caller's own headers are kept; the rester's and the
// token never pass through the CLI.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Seed is one successful request made through an api channel, the only kind of traffic berries ever replays.
type Seed struct {
	ID          string `json:"id"`
	Rester      string `json:"rester"`
	Environment string `json:"environment"`
	User        string `json:"user"`
	Method      string `json:"method"`
	// Path is relative to the environment's base URL, cleaned the way the channel's scope sees it; routes match it.
	Path  string `json:"path"`
	Query string `json:"query,omitempty"`
	// Target is the path as the caller passed it, which a replay sends again so the channel resolves it the same way.
	Target      string    `json:"target"`
	ContentType string    `json:"content_type,omitempty"`
	Body        []byte    `json:"body,omitempty"`
	Headers     []Header  `json:"headers,omitempty"`
	Status      int       `json:"status"`
	Digest      string    `json:"digest"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// key identifies a seed so recording the same call again replaces it instead of piling up copies.
func (s Seed) key() string {
	return strings.Join([]string{s.Rester, s.Environment, s.User, s.Method, s.Path, s.Query}, "\x00")
}

func shortHash(prefix, key string) string {
	sum := sha256.Sum256([]byte(key))
	return prefix + hex.EncodeToString(sum[:])[:7]
}

// SaveSeed records a seed, replacing an earlier recording of the same call. One file per seed keeps concurrent tabs
// from losing each other's writes, since each write replaces only its own file.
func (st Store) SaveSeed(s Seed) (Seed, error) {
	s.ID = shortHash("s", s.key())
	data, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	sum := sha256.Sum256([]byte(s.key()))
	name := hex.EncodeToString(sum[:16]) + ".json"
	if err := atomicfile.Write(filepath.Join(st.Dir, "seeds", name), data, 0o600); err != nil {
		return s, fmt.Errorf("berries: saving seed: %w", err)
	}
	return s, nil
}

// Seeds lists every recorded seed, oldest first. A file that does not decode is skipped, so one damaged seed never
// stops a run.
func (st Store) Seeds() ([]Seed, error) {
	entries, err := os.ReadDir(filepath.Join(st.Dir, "seeds"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("berries: reading seeds: %w", err)
	}
	var seeds []Seed
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(st.Dir, "seeds", e.Name()))
		if err != nil {
			continue
		}
		var s Seed
		if json.Unmarshal(data, &s) == nil && s.ID != "" {
			seeds = append(seeds, s)
		}
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].RecordedAt.Before(seeds[j].RecordedAt) })
	return seeds, nil
}

// Stains reads the stain list, empty when no run has happened yet.
func (st Store) Stains() ([]Stain, error) {
	data, err := os.ReadFile(filepath.Join(st.Dir, "stains.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("berries: reading stains: %w", err)
	}
	var stains []Stain
	if err := json.Unmarshal(data, &stains); err != nil {
		return nil, fmt.Errorf("berries: stains.json does not decode: %w", err)
	}
	return stains, nil
}

// SaveStains replaces the stain list whole.
func (st Store) SaveStains(stains []Stain) error {
	data, err := json.MarshalIndent(stains, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Write(filepath.Join(st.Dir, "stains.json"), data, 0o600); err != nil {
		return fmt.Errorf("berries: saving stains: %w", err)
	}
	return nil
}

// NextRunID numbers runs from 1 by the run files already present.
func (st Store) NextRunID() int {
	entries, _ := os.ReadDir(filepath.Join(st.Dir, "runs"))
	n := 0
	for _, e := range entries {
		var id int
		if _, err := fmt.Sscanf(e.Name(), "%d.json", &id); err == nil && id > n {
			n = id
		}
	}
	return n + 1
}

// SaveRun keeps a run's coverage rows and findings, so a later reader can see what was tested, clean rows included.
func (st Store) SaveRun(r *Run) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.Write(filepath.Join(st.Dir, "runs", fmt.Sprintf("%d.json", r.ID)), data, 0o600); err != nil {
		return fmt.Errorf("berries: saving run: %w", err)
	}
	return nil
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/internal/agentconf"
)

// A record nobody has refreshed for this long belongs to a tab that is gone, and is removed on the next write.
const contextKeep = 7 * 24 * time.Hour

// A tab id becomes a file name, so it is held to the characters lemongrass tab ids use.
var tabIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// Context is one tab's context window use as of its latest reported sample.
type Context struct {
	UsedTokens   int64  `json:"used_tokens"`
	WindowTokens int64  `json:"window_tokens"`
	SeenAt       int64  `json:"seen_at"`
	Vendor       string `json:"vendor,omitempty"`
	// CacheTTL is the prompt cache lifetime in seconds the session's latest cache write asked for, 0 when its
	// transcript never showed one.
	CacheTTL int64 `json:"cache_ttl,omitempty"`
}

func ContextDir(home string) string {
	return filepath.Join(Dir(home), "context")
}

func contextPath(home, tabID string) string {
	return filepath.Join(ContextDir(home), tabID+".json")
}

// RecordContext saves the context_window block of one statusline payload for the tab. Used tokens are the latest
// request's input, cache writes and cache reads, which is what the next request carries. A payload without usage,
// which Claude Code sends before the first response, records nothing, and an unchanged figure is not rewritten,
// because the statusline runs on every update while Claude streams and the app watches these files.
func RecordContext(home, tabID string, payload []byte, now time.Time) error {
	if !tabIDPattern.MatchString(tabID) {
		return nil
	}
	var p struct {
		TranscriptPath string `json:"transcript_path"`
		ContextWindow  *struct {
			Size         int64 `json:"context_window_size"`
			CurrentUsage *struct {
				Input         int64 `json:"input_tokens"`
				CacheCreation int64 `json:"cache_creation_input_tokens"`
				CacheRead     int64 `json:"cache_read_input_tokens"`
			} `json:"current_usage"`
		} `json:"context_window"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("agentusage: parsing statusline payload: %w", err)
	}
	cw := p.ContextWindow
	if cw == nil || cw.Size <= 0 || cw.CurrentUsage == nil {
		return nil
	}
	u := cw.CurrentUsage
	rec := Context{UsedTokens: u.Input + u.CacheCreation + u.CacheRead, WindowTokens: cw.Size, SeenAt: now.Unix()}
	path := contextPath(home, tabID)
	old, oldErr := readContext(path)
	if oldErr == nil && old.UsedTokens == rec.UsedTokens && old.WindowTokens == rec.WindowTokens {
		return nil
	}
	// A turn served wholly from cache writes nothing, so it keeps the lifetime an earlier write asked for.
	rec.CacheTTL = transcriptCacheTTL(p.TranscriptPath)
	if rec.CacheTTL == 0 && oldErr == nil {
		rec.CacheTTL = old.CacheTTL
	}
	return writeContext(home, path, rec, now)
}

func writeContext(home, path string, rec Context, now time.Time) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := agentconf.WriteAtomic(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	pruneContexts(home, now)
	return nil
}

func pruneContexts(home string, now time.Time) {
	entries, err := os.ReadDir(ContextDir(home))
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !strings.HasSuffix(e.Name(), ".json") || now.Sub(info.ModTime()) < contextKeep {
			continue
		}
		os.Remove(filepath.Join(ContextDir(home), e.Name()))
	}
}

func readContext(path string) (Context, error) {
	var rec Context
	data, err := os.ReadFile(path)
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, fmt.Errorf("agentusage: parsing %s: %w", path, err)
	}
	return rec, nil
}

// ReadContexts returns every recorded tab's context by tab id. A file that cannot be read is skipped, since a record
// being replaced mid-read is rewritten again on the tab's next response.
func ReadContexts(home string) (map[string]Context, error) {
	out := map[string]Context{}
	entries, err := os.ReadDir(ContextDir(home))
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		tabID, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !tabIDPattern.MatchString(tabID) {
			continue
		}
		if rec, err := readContext(filepath.Join(ContextDir(home), e.Name())); err == nil {
			out[tabID] = rec
		}
	}
	return out, nil
}

// The transcript tail read for the cache lifetime: the latest assistant turns, never the whole file.
const transcriptTail = 512 << 10

var (
	cache1h = regexp.MustCompile(`"ephemeral_1h_input_tokens":\s*(\d+)`)
	cache5m = regexp.MustCompile(`"ephemeral_5m_input_tokens":\s*(\d+)`)
)

// transcriptCacheTTL reads the cache lifetime, in seconds, of the latest turn in a Claude Code transcript that wrote
// to the prompt cache: each assistant line's usage splits its cache writes into 1h and 5m buckets. It is 0 when no
// turn in the tail wrote any, or the transcript has no such fields.
func transcriptCacheTTL(path string) int64 {
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	start := max(info.Size()-transcriptTail, 0)
	buf := make([]byte, info.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil {
		return 0
	}
	lines := strings.Split(string(buf), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if bucketTokens(cache1h, lines[i]) > 0 {
			return 3600
		}
		if bucketTokens(cache5m, lines[i]) > 0 {
			return 300
		}
	}
	return 0
}

func bucketTokens(re *regexp.Regexp, line string) int64 {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return n
}

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// BackfillCacheTTL fills the cache lifetime of records written before lifetimes were recorded, reading it from the
// tab's Claude Code transcript, which sits under any account's config directory as projects/<slug>/<session>.jsonl.
// sessions maps tab id to session id. A filled record is written back with its other fields unchanged, so the next
// read is cheap; a tab whose transcript is missing or shows no cache write keeps its record as it was.
func BackfillCacheTTL(home string, tabs map[string]Context, sessions map[string]string) {
	for tabID, rec := range tabs {
		sessionID := sessions[tabID]
		if rec.CacheTTL != 0 || !sessionIDPattern.MatchString(sessionID) || !tabIDPattern.MatchString(tabID) {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(home, ".claude*", "projects", "*", sessionID+".jsonl"))
		for _, transcript := range matches {
			ttl := transcriptCacheTTL(transcript)
			if ttl == 0 {
				continue
			}
			rec.CacheTTL = ttl
			if data, err := json.Marshal(rec); err == nil {
				if agentconf.WriteAtomic(contextPath(home, tabID), append(data, '\n'), 0o600) == nil {
					tabs[tabID] = rec
				}
			}
			break
		}
	}
}

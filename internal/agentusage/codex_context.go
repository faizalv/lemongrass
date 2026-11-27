// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const codexVendor = "codex"

// Keep each conversation in its own file so a late write cannot replace another conversation's sample.
func codexContextPath(home, tabID, sessionID string) string {
	return contextPath(home, tabID+"."+sessionID)
}

// RecordCodexContext takes a snapshot of the tab's rollout. Refresh also wakes the context feed
// after a possible session change, even when the resumed conversation's figures are unchanged.
func RecordCodexContext(home, tabID, sessionID, transcriptPath, model string, refresh bool) error {
	if !tabIDPattern.MatchString(tabID) || !sessionIDPattern.MatchString(sessionID) {
		return nil
	}
	path := codexContextPath(home, tabID, sessionID)
	old, oldErr := readContext(path)
	rec, observed := lastCodexContext(transcriptPath, sessionID, model)
	if !observed && oldErr == nil {
		rec = old
	}
	rec.Vendor = codexVendor
	if !refresh && (!observed || (oldErr == nil && old == rec)) {
		return nil
	}
	return writeContext(home, path, rec, time.Now())
}

// MergeCodexContexts selects only snapshots that belong to the tab's currently registered session.
func MergeCodexContexts(home string, tabs map[string]Context, sessions map[string]string) {
	for tabID, sessionID := range sessions {
		if !tabIDPattern.MatchString(tabID) || !sessionIDPattern.MatchString(sessionID) {
			continue
		}
		rec, err := readContext(codexContextPath(home, tabID, sessionID))
		if err != nil || rec.Vendor != codexVendor {
			continue
		}
		if rec.WindowTokens > 0 {
			tabs[tabID] = rec
		} else {
			delete(tabs, tabID)
		}
	}
}

type codexContextLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type  string `json:"type"`
		Model string `json:"model"`
		Info  *struct {
			ModelContextWindow int64 `json:"model_context_window"`
			LastTokenUsage     *struct {
				TotalTokens *int64 `json:"total_tokens"`
			} `json:"last_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

// A compaction or a model change makes an earlier sample inapplicable. If the tail
// contains no usage or reset, keep the last observed sample while the agent runs tools.
func lastCodexContext(path, sessionID, model string) (Context, bool) {
	var rec Context
	name := filepath.Base(path)
	if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, "-"+sessionID+".jsonl") {
		return rec, false
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return rec, false
	}
	tail, _, err := readCodexRolloutTail(path)
	if err != nil {
		return rec, false
	}
	lines := bytes.Split(tail, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		raw := lines[i]
		if !bytes.Contains(raw, []byte(`"token_count"`)) &&
			!bytes.Contains(raw, []byte(`"turn_context"`)) &&
			!bytes.Contains(raw, []byte(`"compacted"`)) &&
			!bytes.Contains(raw, []byte(`"context_compacted"`)) {
			continue
		}
		var line codexContextLine
		if json.Unmarshal(raw, &line) != nil {
			continue
		}
		switch {
		case line.Type == "compacted", line.Type == "event_msg" && line.Payload.Type == "context_compacted":
			return rec, true
		case line.Type == "turn_context" && line.Payload.Model != "":
			if model != "" && line.Payload.Model != model {
				return Context{}, true
			}
			if rec.WindowTokens > 0 {
				return rec, true
			}
		case line.Type == "event_msg" && line.Payload.Type == "token_count" && rec.WindowTokens == 0:
			info := line.Payload.Info
			if info == nil || info.ModelContextWindow <= 0 || info.LastTokenUsage == nil ||
				info.LastTokenUsage.TotalTokens == nil || *info.LastTokenUsage.TotalTokens < 0 {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
			if err == nil {
				rec = Context{
					UsedTokens: *info.LastTokenUsage.TotalTokens, WindowTokens: info.ModelContextWindow,
					SeenAt: at.Unix(), Vendor: codexVendor,
				}
			}
		}
	}
	return rec, rec.WindowTokens > 0
}

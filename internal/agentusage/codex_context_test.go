// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const codexContextTime = "2026-11-27T14:06:01.202Z"

func codexContextEvent(used, window int64) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"model_context_window":%d,"last_token_usage":{"total_tokens":%d},"total_token_usage":{"total_tokens":800000}}}}`+"\n", codexContextTime, window, used)
}

func writeCodexContextRollout(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCodexContextUsesLatestCompleteResponse(t *testing.T) {
	rollout := filepath.Join(t.TempDir(), "rollout-1-session-a.jsonl")
	content := strings.Repeat("x", codexTailBytes) + "\n" +
		`{"type":"turn_context","payload":{"model":"first-model"}}` + "\n" +
		codexContextEvent(90_000, 200_000) +
		codexContextEvent(143_488, 475_000) +
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":`
	writeCodexContextRollout(t, rollout, content)

	at, _ := time.Parse(time.RFC3339Nano, codexContextTime)
	want := Context{UsedTokens: 143_488, WindowTokens: 475_000, SeenAt: at.Unix(), Vendor: "codex"}
	got, ok := lastCodexContext(rollout, "session-a", "first-model")
	if !ok || got != want {
		t.Fatalf("context = %+v, %t; want %+v", got, ok, want)
	}
	if _, ok := lastCodexContext(rollout, "session-b", "first-model"); ok {
		t.Fatalf("another session's rollout produced a sample")
	}
}

func TestCodexContextResetsForModelChangeAndCompaction(t *testing.T) {
	home := t.TempDir()
	rollout := filepath.Join(home, "rollout-1-session-a.jsonl")
	firstTurn := `{"type":"turn_context","payload":{"model":"first-model"}}` + "\n" + codexContextEvent(190_000, 200_000)
	secondTurn := `{"type":"turn_context","payload":{"model":"second-model"}}` + "\n"
	compaction := `{"type":"compacted","payload":{}}` + "\n"
	tests := []struct {
		name         string
		content      string
		model        string
		used, window int64
	}{
		{"first response", firstTurn, "first-model", 190_000, 200_000},
		{"waiting for the new model", firstTurn + secondTurn, "second-model", 0, 0},
		{"new model response", firstTurn + secondTurn + codexContextEvent(50_000, 100_000), "second-model", 50_000, 100_000},
		{"waiting after compaction", firstTurn + compaction, "first-model", 0, 0},
		{"post-compaction sample", firstTurn + compaction + codexContextEvent(22_000, 200_000), "first-model", 22_000, 200_000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writeCodexContextRollout(t, rollout, tc.content)
			if err := RecordCodexContext(home, "tab-a", "session-a", rollout, tc.model, false); err != nil {
				t.Fatal(err)
			}
			tabs := map[string]Context{}
			MergeCodexContexts(home, tabs, map[string]string{"tab-a": "session-a"})
			got, shown := tabs["tab-a"]
			if shown != (tc.window > 0) || got.UsedTokens != tc.used || got.WindowTokens != tc.window {
				t.Fatalf("context = %+v, %t; want %d/%d", got, shown, tc.used, tc.window)
			}
		})
	}
}

func TestCodexContextFollowsEachTabsCurrentConversation(t *testing.T) {
	home := t.TempDir()
	record := func(tab, session string, used int64) {
		t.Helper()
		rollout := filepath.Join(home, "rollout-1-"+session+".jsonl")
		writeCodexContextRollout(t, rollout, codexContextEvent(used, 400_000))
		if err := RecordCodexContext(home, tab, session, rollout, "", true); err != nil {
			t.Fatal(err)
		}
	}
	record("tab-a", "session-a", 100_000)
	record("tab-b", "session-b", 200_000)
	record("tab-a", "session-c", 300_000)
	record("tab-a", "session-a", 350_000)

	sessions := map[string]string{"tab-a": "session-c", "tab-b": "session-b"}
	tabs := map[string]Context{}
	MergeCodexContexts(home, tabs, sessions)
	if a, b := tabs["tab-a"], tabs["tab-b"]; a.UsedTokens != 300_000 || b.UsedTokens != 200_000 ||
		a.CacheTTL != 0 || b.CacheTTL != 0 {
		t.Fatalf("unexpected active session contexts: %+v", tabs)
	}

	sessions["tab-a"] = "session-d"
	tabs = map[string]Context{}
	MergeCodexContexts(home, tabs, sessions)
	if _, exists := tabs["tab-a"]; exists {
		t.Fatalf("new conversation inherited an old sample: %+v", tabs)
	}
	sessions["tab-a"] = "session-a"
	MergeCodexContexts(home, tabs, sessions)
	if got := tabs["tab-a"].UsedTokens; got != 350_000 {
		t.Fatalf("resumed session context = %d; want 350000", got)
	}
}

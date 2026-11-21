// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const tabA = "ba25ec5b-a010-43d8-9886-726d6b485b65"

func contextPayload(input, creation, read int64) []byte {
	return []byte(`{"session_id":"s","context_window":{"context_window_size":1000000,"current_usage":{"input_tokens":` +
		itoa(input) + `,"output_tokens":22,"cache_creation_input_tokens":` + itoa(creation) +
		`,"cache_read_input_tokens":` + itoa(read) + `},"used_percentage":5}}`)
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestRecordContextSumsInputAndCache(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1793170000, 0)
	if err := RecordContext(home, tabA, contextPayload(2, 45204, 1000), now); err != nil {
		t.Fatal(err)
	}
	got, err := ReadContexts(home)
	if err != nil {
		t.Fatal(err)
	}
	want := Context{UsedTokens: 46206, WindowTokens: 1000000, SeenAt: now.Unix()}
	if got[tabA] != want {
		t.Errorf("context = %+v, want %+v", got[tabA], want)
	}
}

func TestRecordContextSkipsPayloadWithoutUsage(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{
		`{"session_id":"s"}`,
		`{"context_window":{"context_window_size":200000,"current_usage":null}}`,
		`{"context_window":{"context_window_size":0,"current_usage":{"input_tokens":5}}}`,
	} {
		if err := RecordContext(home, tabA, []byte(p), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := ReadContexts(home)
	if len(got) != 0 {
		t.Errorf("recorded %v", got)
	}
}

func TestRecordContextRejectsUnsafeTabID(t *testing.T) {
	home := t.TempDir()
	for _, id := range []string{"", "../escape", "a/b", "tab.id"} {
		if err := RecordContext(home, id, contextPayload(1, 1, 1), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(ContextDir(home)); !os.IsNotExist(err) {
		t.Errorf("context dir was created for an unsafe tab id: %v", err)
	}
}

func TestRecordContextLeavesUnchangedFigureAlone(t *testing.T) {
	home := t.TempDir()
	first := time.Unix(1793170000, 0)
	RecordContext(home, tabA, contextPayload(1, 100, 0), first)
	RecordContext(home, tabA, contextPayload(1, 100, 0), first.Add(time.Minute))
	got, _ := ReadContexts(home)
	if got[tabA].SeenAt != first.Unix() {
		t.Errorf("unchanged figure was rewritten: seen_at %d", got[tabA].SeenAt)
	}
	RecordContext(home, tabA, contextPayload(1, 100, 50), first.Add(2*time.Minute))
	got, _ = ReadContexts(home)
	if got[tabA].UsedTokens != 151 {
		t.Errorf("changed figure not recorded: %+v", got[tabA])
	}
}

func TestRecordContextPrunesOldRecords(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1793170000, 0)
	stale := "11111111-2222-3333-4444-555555555555"
	RecordContext(home, stale, contextPayload(1, 1, 1), now)
	old := now.Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(ContextDir(home), stale+".json"), old, old); err != nil {
		t.Fatal(err)
	}
	RecordContext(home, tabA, contextPayload(1, 1, 1), now)
	got, _ := ReadContexts(home)
	if _, ok := got[stale]; ok {
		t.Error("a record untouched for 8 days was kept")
	}
	if _, ok := got[tabA]; !ok {
		t.Error("the fresh record is missing")
	}
}

func TestReadContextsWithoutDirectory(t *testing.T) {
	got, err := ReadContexts(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func payloadWithTranscript(path string, input int64) []byte {
	return []byte(`{"transcript_path":"` + path + `","context_window":{"context_window_size":1000000,"current_usage":{"input_tokens":` +
		itoa(input) + `,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`)
}

func TestRecordContextReadsCacheTTLFromTranscript(t *testing.T) {
	home := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "s.jsonl")
	write := func(lines string) {
		if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ttl := func() int64 {
		got, _ := ReadContexts(home)
		return got[tabA].CacheTTL
	}
	write(`{"type":"assistant","message":{"usage":{"cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":900}}}}` + "\n" +
		`{"type":"assistant","message":{"usage":{"cache_creation":{"ephemeral_1h_input_tokens":1116,"ephemeral_5m_input_tokens":0}}}}` + "\n" +
		`{"type":"user","message":{"content":"next"}}` + "\n")
	if err := RecordContext(home, tabA, payloadWithTranscript(transcript, 1), time.Now()); err != nil {
		t.Fatal(err)
	}
	if ttl() != 3600 {
		t.Fatalf("ttl = %d, want 3600", ttl())
	}

	// A later turn that wrote nothing keeps the lifetime already recorded.
	write(`{"type":"assistant","message":{"usage":{"cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":0}}}}` + "\n")
	if err := RecordContext(home, tabA, payloadWithTranscript(transcript, 2), time.Now()); err != nil {
		t.Fatal(err)
	}
	if ttl() != 3600 {
		t.Fatalf("ttl after a cache-only turn = %d, want 3600", ttl())
	}

	write(`{"type":"assistant","message":{"usage":{"cache_creation":{"ephemeral_5m_input_tokens":40,"ephemeral_1h_input_tokens":0}}}}` + "\n")
	if err := RecordContext(home, tabA, payloadWithTranscript(transcript, 3), time.Now()); err != nil {
		t.Fatal(err)
	}
	if ttl() != 300 {
		t.Fatalf("ttl = %d, want 300", ttl())
	}
}

func TestBackfillCacheTTLReadsTheSessionTranscript(t *testing.T) {
	home := t.TempDir()
	write := func(tab string, rec Context) {
		if err := os.MkdirAll(ContextDir(home), 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(rec)
		if err := os.WriteFile(contextPath(home, tab), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const tabB = "0d6f3a52-9c1e-4c55-9b0f-6f1f2a3b4c5d"
	const tabC = "7e1a2b3c-4d5e-4f60-8172-839405162738"
	old := Context{UsedTokens: 300000, WindowTokens: 1000000, SeenAt: 100}
	write(tabA, old)
	write(tabB, old)
	write(tabC, Context{UsedTokens: 1, WindowTokens: 1, SeenAt: 1, CacheTTL: 300})
	dir := filepath.Join(home, ".claude-second", "projects", "-some-project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"message":{"usage":{"cache_creation":{"ephemeral_1h_input_tokens":12,"ephemeral_5m_input_tokens":0}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sess-a.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	tabs, _ := ReadContexts(home)
	BackfillCacheTTL(home, tabs, map[string]string{tabA: "sess-a", tabB: "sess-missing", tabC: "sess-a"})
	if tabs[tabA].CacheTTL != 3600 || tabs[tabA].SeenAt != 100 {
		t.Errorf("tab a = %+v, want ttl 3600 with seen_at kept", tabs[tabA])
	}
	if tabs[tabB].CacheTTL != 0 {
		t.Errorf("tab b without a transcript = %+v", tabs[tabB])
	}
	if tabs[tabC].CacheTTL != 300 {
		t.Errorf("a recorded lifetime was replaced: %+v", tabs[tabC])
	}
	again, _ := ReadContexts(home)
	if again[tabA].CacheTTL != 3600 {
		t.Errorf("the filled lifetime was not written back: %+v", again[tabA])
	}
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

// The fixture preserves the observed agy 1.5.3 compacted transcript's record
// metadata and types. User text, orientation, laws and summary content are sanitized.
func agyCompactionFixture(t *testing.T) (hookEvent, []byte) {
	t.Helper()
	data, err := os.ReadFile("testdata/agy-compaction.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	root := projectWithBiblio(t)
	if err := os.MkdirAll(filepath.Join(root, "biblio", "laws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "biblio", "laws", "summary.md"), []byte("# Laws summary\n- example law"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".lemongrass", "orientation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"tab", "thinker"} {
		if err := os.WriteFile(filepath.Join(dir, "agy-"+role+".md"), []byte(role+" orientation"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := agyCapture(t, "PreInvocation", root, nil)
	p.TranscriptPath = filepath.Join(filepath.Dir(path), "transcript_full.jsonl")
	return p, data
}

func appendAgyTranscript(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
}

func TestAgyCompactionRestoresOrientationOnceAndResumeStaysLawsOnly(t *testing.T) {
	store := openHookTestStore(t)
	p, data := agyCompactionFixture(t)
	path := agyTranscriptPath(p.TranscriptPath)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	checkpoint := append(append([]byte{}, lines[len(lines)-1]...), '\n')
	before := append(bytes.Join(lines[:len(lines)-1], []byte("\n")), '\n')
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(agyResumedEnv, "launch-a")
	plain := hookUserPromptSubmit(store, p)
	if strings.Contains(plain.AdditionalContext, "orientation") || !strings.Contains(plain.AdditionalContext, "example law") {
		t.Fatalf("plain resume = %+v", plain)
	}
	appendAgyTranscript(t, path, checkpoint)
	compacted := hookUserPromptSubmit(store, p)
	if !strings.HasPrefix(compacted.AdditionalContext, "tab orientation\n\n# Laws summary") || strings.Count(compacted.AdditionalContext, "example law") != 1 {
		t.Fatalf("compacted = %+v", compacted)
	}
	if again := hookUserPromptSubmit(store, p); again.hasOutput() {
		t.Fatalf("compaction repeated = %+v", again)
	}
	t.Setenv(agyResumedEnv, "launch-b")
	resumed := hookUserPromptSubmit(store, p)
	if strings.Contains(resumed.AdditionalContext, "orientation") || strings.Count(resumed.AdditionalContext, "example law") != 1 {
		t.Fatalf("resume after restored compaction = %+v", resumed)
	}
	var next agyTranscriptStep
	if err := json.Unmarshal(checkpoint, &next); err != nil {
		t.Fatal(err)
	}
	next.Index++
	next.CreatedAt = "2026-11-08T06:00:00Z"
	more, _ := json.Marshal(next)
	appendAgyTranscript(t, path, append(more, '\n'))
	p.Continuation = true
	second := hookUserPromptSubmit(store, p)
	if !strings.HasPrefix(second.AdditionalContext, "tab orientation\n\n# Laws summary") {
		t.Fatalf("second compaction mid-turn = %+v", second)
	}
	if again := hookUserPromptSubmit(store, p); again.hasOutput() {
		t.Fatalf("second compaction repeated = %+v", again)
	}
}

func TestAgyCompactionWithNewResumeTokenDoesNotDuplicateLaws(t *testing.T) {
	store := openHookTestStore(t)
	p, _ := agyCompactionFixture(t)
	t.Setenv(agyResumedEnv, "new-launch")
	result := hookUserPromptSubmit(store, p)
	if !strings.Contains(result.AdditionalContext, "tab orientation") || strings.Count(result.AdditionalContext, "example law") != 1 {
		t.Fatalf("compaction and resume = %+v", result)
	}
	if again := hookUserPromptSubmit(store, p); again.hasOutput() {
		t.Fatalf("resume token was not consumed: %+v", again)
	}
}

func TestAgyCompactionRestoresThinkerRoleAndKeepsPendingNotes(t *testing.T) {
	store := storeWithPendingMention(t)
	_, err := store.CreateGroup("review", session.Member{TabID: notifyTabAuthor, Label: "lead", Vendor: "claude"}, []session.Member{{TabID: notifyTabTarget, Label: "tester", Vendor: "agy"}})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := agyCompactionFixture(t)
	p.TabID, p.Continuation = notifyTabTarget, true
	store.MarkReady(p.TabID, session.ThinkerStartMark)
	result := hookUserPromptSubmit(store, p)
	if !strings.HasPrefix(result.AdditionalContext, "thinker orientation\n\n# Laws summary") || !strings.Contains(result.AdditionalContext, ": 1 for you") {
		t.Fatalf("thinker compacted = %+v", result)
	}
	if marks, _ := store.Marks(p.TabID); !marks[session.ThinkerStartMark] {
		t.Fatal("compaction reset readiness marks")
	}
	if id := tabSessionOf(t, store, p.TabID); id != "" {
		t.Fatalf("continuation recorded a prompt session: %s", id)
	}
}

func TestAgyCompactionTranscriptFailuresAndMissingOrientationFailSoft(t *testing.T) {
	for _, kind := range []string{"missing", "malformed", "directory", "missing orientation"} {
		t.Run(kind, func(t *testing.T) {
			store := openHookTestStore(t)
			p, _ := agyCompactionFixture(t)
			path := agyTranscriptPath(p.TranscriptPath)
			switch kind {
			case "missing":
				p.TranscriptPath = filepath.Join(t.TempDir(), "absent.jsonl")
			case "malformed":
				if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				p.TranscriptPath = t.TempDir()
			case "missing orientation":
				t.Setenv("HOME", t.TempDir())
			}
			t.Setenv(agyResumedEnv, "resumed-launch")
			result := hookUserPromptSubmit(store, p)
			if !strings.HasPrefix(result.AdditionalContext, "# Laws summary") || strings.Contains(result.AdditionalContext, "orientation") {
				t.Fatalf("failed transcript/orientation = %+v", result)
			}
		})
	}
}

func TestAgyTranscriptScanIsIncrementalOnLargeHistory(t *testing.T) {
	data, err := os.ReadFile("testdata/agy-compaction.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	line := []byte(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"` + strings.Repeat("x", 8192) + "\"}\n")
	large := bytes.Repeat(line, 2048)
	large = append(large, data...)
	if err := os.WriteFile(path, large, 0o600); err != nil {
		t.Fatal(err)
	}
	scan := func(previous agyTranscriptCursor) (agyTranscriptCursor, int64) {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		state, read, err := agyScanTranscript(f, previous)
		if err != nil {
			t.Fatal(err)
		}
		return state, read
	}
	state, read := scan(agyTranscriptCursor{})
	if state.Checkpoint == "" || read != int64(len(large)) {
		t.Fatalf("cold scan = %+v, %d", state, read)
	}
	unchanged, read := scan(state)
	if unchanged != state || read != 0 {
		t.Fatalf("unchanged history re-read: %d bytes", read)
	}
	appendAgyTranscript(t, path, line)
	appended, read := scan(state)
	if read != int64(len(line)) || appended.Checkpoint != state.Checkpoint {
		t.Fatalf("appended scan read %d, want %d", read, len(line))
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	truncated, read := scan(appended)
	if read != int64(len(data)) || truncated.Checkpoint == "" {
		t.Fatalf("truncated scan = %+v, %d", truncated, read)
	}
}

func TestAgyTranscriptScanRetriesPartialRecordsAndDetectsRewrites(t *testing.T) {
	data, err := os.ReadFile("testdata/agy-compaction.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	checkpoint := lines[len(lines)-1]
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	prefix := []byte("not json\n")
	if err := os.WriteFile(path, append(append([]byte{}, prefix...), checkpoint[:30]...), 0o600); err != nil {
		t.Fatal(err)
	}
	scan := func(previous agyTranscriptCursor) agyTranscriptCursor {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		state, _, err := agyScanTranscript(f, previous)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	state := scan(agyTranscriptCursor{})
	if state.Offset != int64(len(prefix)) || state.Checkpoint != "" {
		t.Fatalf("partial record consumed: %+v", state)
	}
	appendAgyTranscript(t, path, append(append([]byte{}, checkpoint[30:]...), '\n'))
	state = scan(state)
	if state.Checkpoint == "" {
		t.Fatal("completed partial record was missed")
	}
	rewritten := []byte(strings.Repeat(" ", int(state.Offset)-1) + "\n")
	if err := os.WriteFile(path, rewritten, 0o600); err != nil {
		t.Fatal(err)
	}
	if state = scan(state); state.Checkpoint != "" {
		t.Fatalf("rewritten history kept old checkpoint: %+v", state)
	}
}

func TestAgyTranscriptCursorPersistsWithoutTranscriptText(t *testing.T) {
	store := openHookTestStore(t)
	p, _ := agyCompactionFixture(t)
	short := agyTranscriptPath(p.TranscriptPath)
	first := agyLatestCheckpoint(p)
	if first == "" {
		t.Fatal("short transcript checkpoint missing")
	}
	if again := agyLatestCheckpoint(p); again != first {
		t.Fatalf("cached checkpoint changed: %s", again)
	}
	home, _ := os.UserHomeDir()
	paths, err := filepath.Glob(filepath.Join(home, ".lemongrass", "hook-state", "agy", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("cursor files = %v, %v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var state agyTranscriptCursor
	if err := json.Unmarshal(data, &state); err != nil || state.Offset == 0 || state.Checkpoint != first {
		t.Fatalf("cursor = %+v, %v", state, err)
	}
	if strings.Contains(string(data), "Generic") || strings.Contains(string(data), "summary") || strings.Contains(string(data), "content") {
		t.Fatalf("cursor leaked transcript content: %s", data)
	}
	info, err := os.Stat(paths[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("cursor mode = %v, %v", info, err)
	}
	if err := os.WriteFile(paths[0], []byte(`{"offset":999999999,broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	if recovered := agyLatestCheckpoint(p); recovered != first {
		t.Fatalf("corrupt cursor lost checkpoint: %s", recovered)
	}
	if err := os.Rename(short, p.TranscriptPath); err != nil {
		t.Fatal(err)
	}
	if fallback := agyLatestCheckpoint(p); fallback != first {
		t.Fatalf("full-transcript fallback = %s", fallback)
	}
	p.TabID = ""
	if _, renewed := agyCompactionContext(store, p); renewed {
		t.Fatal("non-lemongrass process was oriented")
	}
}

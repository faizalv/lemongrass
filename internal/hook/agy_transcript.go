// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/faizalv/lemongrass/internal/atomicfile"
	"github.com/faizalv/lemongrass/internal/session"
)

type agyTranscriptCursor struct {
	Offset       int64  `json:"offset"`
	HeadHash     string `json:"head_hash"`
	BoundaryHash string `json:"boundary_hash"`
	Checkpoint   string `json:"checkpoint"`
}

type agyTranscriptStep struct {
	Index     int64  `json:"step_index"`
	Source    string `json:"source"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
}

func agyFingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func agyTranscriptPath(path string) string {
	if filepath.Base(path) == "transcript_full.jsonl" {
		short := filepath.Join(filepath.Dir(path), "transcript.jsonl")
		if info, err := os.Stat(short); err == nil && info.Mode().IsRegular() {
			return short
		}
	}
	return path
}

func agyTranscriptRangeHash(file *os.File, start, end int64) string {
	if start < 0 {
		start = 0
	}
	data := make([]byte, end-start)
	if _, err := file.ReadAt(data, start); err != nil {
		return ""
	}
	return agyFingerprint(data)
}

// A hook is a new process each time. Keep only its scan position and hashes on disk,
// and verify both ends of the scanned prefix in case the CLI rewrites or truncates it.
func agyScanTranscript(file *os.File, previous agyTranscriptCursor) (agyTranscriptCursor, int64, error) {
	info, err := file.Stat()
	if err != nil {
		return previous, 0, err
	}
	if !info.Mode().IsRegular() {
		return previous, 0, os.ErrInvalid
	}
	state := previous
	if state.Offset < 0 || state.Offset > info.Size() || state.HeadHash != agyTranscriptRangeHash(file, 0, min(state.Offset, 256)) || state.BoundaryHash != agyTranscriptRangeHash(file, max(state.Offset-256, 0), state.Offset) {
		state = agyTranscriptCursor{}
	}
	if _, err := file.Seek(state.Offset, io.SeekStart); err != nil {
		return previous, 0, err
	}
	reader := bufio.NewReader(io.LimitReader(file, info.Size()-state.Offset))
	var read int64
	for {
		line, err := reader.ReadBytes('\n')
		read += int64(len(line))
		if err != nil {
			// A partial last record is retried at the next invocation, never checkpointed as consumed.
			if err != io.EOF {
				return previous, read, err
			}
			break
		}
		state.Offset += int64(len(line))
		var step agyTranscriptStep
		if json.Unmarshal(line, &step) != nil || step.Source != "SYSTEM" || step.Type != "CHECKPOINT" || step.Status != "DONE" {
			continue
		}
		canonical, _ := json.Marshal(step)
		state.Checkpoint = agyFingerprint(canonical)
	}
	state.HeadHash = agyTranscriptRangeHash(file, 0, min(state.Offset, 256))
	state.BoundaryHash = agyTranscriptRangeHash(file, max(state.Offset-256, 0), state.Offset)
	return state, read, nil
}

func agyLatestCheckpoint(payload hookEvent) string {
	if payload.TranscriptPath == "" || payload.SessionID == "" {
		return ""
	}
	path := agyTranscriptPath(payload.TranscriptPath)
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	key := agyFingerprint([]byte(payload.SessionID + "\n" + path))
	cachePath := filepath.Join(home, ".lemongrass", "hook-state", "agy", key+".json")
	var previous agyTranscriptCursor
	if data, err := os.ReadFile(cachePath); err == nil {
		if json.Unmarshal(data, &previous) != nil {
			previous = agyTranscriptCursor{}
		}
	}
	state, _, err := agyScanTranscript(file, previous)
	if err != nil {
		return ""
	}
	if state != previous {
		data, _ := json.Marshal(state)
		if err := atomicfile.Write(cachePath, data, 0o600); err != nil {
			return ""
		}
	}
	return state.Checkpoint
}

func agyCompactionContext(store *session.Store, payload hookEvent) ([]string, bool) {
	if payload.TabID == "" {
		return nil, false
	}
	checkpoint := agyLatestCheckpoint(payload)
	if checkpoint == "" {
		return nil, false
	}
	mark := "agy-compaction:" + agyFingerprint([]byte(payload.SessionID)) + ":" + checkpoint
	marks, err := store.Marks(payload.TabID)
	if err != nil || marks[mark] {
		return nil, false
	}
	if err := store.MarkReady(payload.TabID, mark); err != nil {
		return nil, false
	}
	projectDir := payload.ProjectDir
	if projectDir == "" {
		projectDir = payload.Cwd
	}
	return []string{agyOrientation(store, payload.TabID), lawsSummary(projectDir)}, true
}

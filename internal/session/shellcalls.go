// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"database/sql"
	"errors"
	"time"
)

// A start whose PostToolUse never came, after a crash or an interrupt, is dropped once it is this old.
const shellCallKeep = 24 * time.Hour

// StartShellCall records when a shell tool call began. callID is the vendor's tool call id, or empty for a vendor that
// sends none, which then keeps one pending call per session.
func (s *Store) StartShellCall(sessionID, callID string, at time.Time) error {
	cutoff := time.Now().Add(-shellCallKeep).UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`DELETE FROM lg_shell_calls WHERE project_id = ? AND started_at < ?`, s.projectID, cutoff); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO lg_shell_calls (project_id, session_id, call_id, started_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (project_id, session_id, call_id) DO UPDATE SET started_at = excluded.started_at
	`, s.projectID, sessionID, callID, at.UTC().Format(time.RFC3339Nano))
	return err
}

// FinishShellCall removes the call's start and returns it; ok is false when none was recorded.
func (s *Store) FinishShellCall(sessionID, callID string) (time.Time, bool, error) {
	var started string
	err := s.db.QueryRow(`
		DELETE FROM lg_shell_calls WHERE project_id = ? AND session_id = ? AND call_id = ? RETURNING started_at
	`, s.projectID, sessionID, callID).Scan(&started)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	at, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return time.Time{}, false, err
	}
	return at, true, nil
}

// DropShellCalls forgets the session's pending starts, since a new or resumed session has no call in flight.
func (s *Store) DropShellCalls(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM lg_shell_calls WHERE project_id = ? AND session_id = ?`, s.projectID, sessionID)
	return err
}

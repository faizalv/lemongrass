// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"database/sql"
)

const (
	StateWorking   = "working"
	StateIdle      = "idle"
	StatePrompting = "prompting"
)

// Which of the agent's own hook events last fired for the tab: a turn is running, the turn ended, or a permission prompt is open.
func (s *Store) SetTabState(tabID, state string) error {
	_, err := s.db.Exec(`
		INSERT INTO lg_tab_state (tab_id, state, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (tab_id) DO UPDATE SET state = excluded.state, updated_at = excluded.updated_at
	`, tabID, state, now())
	return err
}

func (s *Store) ClearTabState(tabID string) error {
	_, err := s.db.Exec(`DELETE FROM lg_tab_state WHERE tab_id = ?`, tabID)
	return err
}

// Empty string, no error, when no hook has recorded a state for the tab.
func (s *Store) TabState(tabID string) (string, error) {
	var state string
	err := s.db.QueryRow(`SELECT state FROM lg_tab_state WHERE tab_id = ?`, tabID).Scan(&state)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return state, err
}

// True unless a permission prompt or question picker is open, where a typed Enter would answer it. A busy agent queues typed input for its next tool call, so working does not block.
func (s *Store) TabReadyForNudge(tabID string) (bool, error) {
	state, err := s.TabState(tabID)
	if err != nil {
		return false, err
	}
	return state != StatePrompting, nil
}

package session

import (
	"database/sql"
	"time"
)

// A tab record is refreshed on spawn and on hook activity, so one untouched for this long belongs to a tab that no longer exists.
const tabRecordTTL = 30 * 24 * time.Hour

func (s *Store) RecordTabSession(tabID, sessionID string) error {
	_, err := s.db.Exec(`
		INSERT INTO lg_tab_sessions (project_id, tab_id, session_id, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (project_id, tab_id) DO UPDATE SET
			session_id = excluded.session_id,
			updated_at = excluded.updated_at
	`, s.projectID, tabID, sessionID, now())
	return err
}

// Maps tab id to the tab's most recently reported session id.
func (s *Store) TabSessions() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT tab_id, session_id FROM lg_tab_sessions WHERE project_id = ?`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var tabID, sessionID string
		if err := rows.Scan(&tabID, &sessionID); err != nil {
			return nil, err
		}
		out[tabID] = sessionID
	}
	return out, rows.Err()
}

// Empty string, no error, when the tab has reported no session.
func (s *Store) SessionForTab(tabID string) (string, error) {
	var sessionID string
	err := s.db.QueryRow(`SELECT session_id FROM lg_tab_sessions WHERE project_id = ? AND tab_id = ?`, s.projectID, tabID).Scan(&sessionID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return sessionID, err
}

func (s *Store) ForgetTabSession(tabID string) error {
	_, err := s.db.Exec(`DELETE FROM lg_tab_sessions WHERE project_id = ? AND tab_id = ?`, s.projectID, tabID)
	return err
}

// Also prunes every project's stale records, so a project that is never opened again does not keep its rows.
func (s *Store) RegisterTab(tabID, vendor string) error {
	ts := now()
	if _, err := s.db.Exec(`
		INSERT INTO lg_tabs (project_id, tab_id, vendor, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (project_id, tab_id) DO UPDATE SET
			vendor = excluded.vendor,
			updated_at = excluded.updated_at
	`, s.projectID, tabID, vendor, ts); err != nil {
		return err
	}
	cutoff := time.Now().Add(-tabRecordTTL).UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`DELETE FROM lg_tabs WHERE updated_at < ?`, cutoff)
	return err
}

// Updates only, so a tab that was never registered stays unrecorded.
func (s *Store) TouchTab(tabID string) error {
	_, err := s.db.Exec(`UPDATE lg_tabs SET updated_at = ? WHERE project_id = ? AND tab_id = ?`, now(), s.projectID, tabID)
	return err
}

// Empty string, no error, when the tab has no record.
func (s *Store) TabVendor(tabID string) (string, error) {
	var vendor string
	err := s.db.QueryRow(`SELECT vendor FROM lg_tabs WHERE project_id = ? AND tab_id = ?`, s.projectID, tabID).Scan(&vendor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return vendor, err
}

func (s *Store) ForgetTab(tabID string) error {
	if err := s.ForgetTabSession(tabID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM lg_tabs WHERE project_id = ? AND tab_id = ?`, s.projectID, tabID)
	return err
}

// Maps tab id to vendor for every registered tab in this project.
func (s *Store) TabVendors() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT tab_id, vendor FROM lg_tabs WHERE project_id = ?`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var tabID, vendor string
		if err := rows.Scan(&tabID, &vendor); err != nil {
			return nil, err
		}
		out[tabID] = vendor
	}
	return out, rows.Err()
}

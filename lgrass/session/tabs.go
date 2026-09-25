package session

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

func (s *Store) ForgetTabSession(tabID string) error {
	_, err := s.db.Exec(`DELETE FROM lg_tab_sessions WHERE project_id = ? AND tab_id = ?`, s.projectID, tabID)
	return err
}

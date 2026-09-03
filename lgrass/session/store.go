package session

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store tracks one project's sessions and recent file activity, both fed
// by Claude Code's own hook events (SessionStart/SessionEnd/PreToolUse/
// PostToolUse) rather than any live process state.
type Store struct {
	db        *sql.DB
	projectID string
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	project_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	started_at TEXT NOT NULL,
	ended_at TEXT,
	last_activity_at TEXT NOT NULL,
	nudge_counter INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (project_id, session_id)
);
CREATE INDEX IF NOT EXISTS idx_sessions_open ON sessions(project_id, ended_at);
CREATE TABLE IF NOT EXISTS file_activity (
	project_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	file_path TEXT NOT NULL,
	dir TEXT NOT NULL,
	touched_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_file_activity_file ON file_activity(project_id, file_path, touched_at);
CREATE INDEX IF NOT EXISTS idx_file_activity_dir ON file_activity(project_id, dir, touched_at);
`

// Open opens (creating and migrating if needed) the shared session
// database at dbPath, scoped to one project. This is very often the
// first lgrass call in a project at all -- a hook fires before any
// knowledge command ever runs -- so it can't assume config.Dir() exists
// yet.
func Open(dbPath, projectID string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(dbPath), err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return &Store{db: db, projectID: projectID}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Start records a session beginning, or restarting under a reused id
// (ended_at and nudge_counter reset either way).
func (s *Store) Start(sessionID string) error {
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO sessions (project_id, session_id, started_at, ended_at, last_activity_at, nudge_counter)
		VALUES (?, ?, ?, NULL, ?, 0)
		ON CONFLICT (project_id, session_id) DO UPDATE SET
			started_at = excluded.started_at,
			ended_at = NULL,
			last_activity_at = excluded.last_activity_at,
			nudge_counter = 0
	`, s.projectID, sessionID, ts, ts)
	return err
}

// Touch bumps a session's last-activity timestamp. It upserts the row if
// no SessionStart was ever recorded for it, so a missed or misconfigured
// SessionStart hook doesn't leave liveness permanently blind to it.
func (s *Store) Touch(sessionID string) error {
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO sessions (project_id, session_id, started_at, ended_at, last_activity_at, nudge_counter)
		VALUES (?, ?, ?, NULL, ?, 0)
		ON CONFLICT (project_id, session_id) DO UPDATE SET
			last_activity_at = excluded.last_activity_at
	`, s.projectID, sessionID, ts, ts)
	return err
}

// End marks a session as no longer live. It no longer counts toward
// population, liveness, or collision checks.
func (s *Store) End(sessionID string) error {
	_, err := s.db.Exec(`UPDATE sessions SET ended_at = ? WHERE project_id = ? AND session_id = ?`, now(), s.projectID, sessionID)
	return err
}

// LogFileActivity records that a session just wrote or edited filePath,
// for other sessions' collision checks to query against.
func (s *Store) LogFileActivity(sessionID, filePath string) error {
	_, err := s.db.Exec(`
		INSERT INTO file_activity (project_id, session_id, file_path, dir, touched_at)
		VALUES (?, ?, ?, ?, ?)
	`, s.projectID, sessionID, filePath, filepath.Dir(filePath), now())
	return err
}

// ActivityHit is one other live session's recent touch on a file or its
// parent directory.
type ActivityHit struct {
	SessionID string
	FilePath  string
	TouchedAt string
	SameFile  bool // false means it was a sibling file in the same directory
}

// RecentActivity returns, most recent first and deduplicated to one hit
// per session, every other currently-live session that touched filePath
// or a sibling file in its immediate parent directory within window.
func (s *Store) RecentActivity(excludeSessionID, filePath string, window time.Duration) ([]ActivityHit, error) {
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
	dir := filepath.Dir(filePath)

	rows, err := s.db.Query(`
		SELECT fa.session_id, fa.file_path, fa.touched_at
		FROM file_activity fa
		JOIN sessions sess ON sess.project_id = fa.project_id AND sess.session_id = fa.session_id
		WHERE fa.project_id = ?
			AND fa.session_id != ?
			AND sess.ended_at IS NULL
			AND fa.touched_at > ?
			AND (fa.file_path = ? OR fa.dir = ?)
		ORDER BY fa.touched_at DESC
	`, s.projectID, excludeSessionID, cutoff, filePath, dir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var hits []ActivityHit
	for rows.Next() {
		var h ActivityHit
		if err := rows.Scan(&h.SessionID, &h.FilePath, &h.TouchedAt); err != nil {
			return nil, err
		}
		if seen[h.SessionID] {
			continue
		}
		seen[h.SessionID] = true
		h.SameFile = h.FilePath == filePath
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// SessionStatus is one other live session's liveness for reporting.
type SessionStatus struct {
	SessionID string
	Active    bool // false means idling: no tool activity within the threshold
}

// Liveness returns every other currently-live session in the project,
// most recently active first, with Active derived from last_activity_at
// against idleThreshold. Its length is also the population count (how
// many other sessions are open right now).
func (s *Store) Liveness(excludeSessionID string, idleThreshold time.Duration) ([]SessionStatus, error) {
	cutoff := time.Now().Add(-idleThreshold).UTC().Format(time.RFC3339)

	rows, err := s.db.Query(`
		SELECT session_id, last_activity_at FROM sessions
		WHERE project_id = ? AND ended_at IS NULL AND session_id != ?
		ORDER BY last_activity_at DESC
	`, s.projectID, excludeSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionStatus
	for rows.Next() {
		var id, lastActivity string
		if err := rows.Scan(&id, &lastActivity); err != nil {
			return nil, err
		}
		out = append(out, SessionStatus{SessionID: id, Active: lastActivity > cutoff})
	}
	return out, rows.Err()
}

// IncrementNudgeCounter bumps a session's tool-call counter and reports
// whether it just crossed threshold, resetting it to 0 when it does. It
// upserts the session row if missing, for the same reason Touch does.
func (s *Store) IncrementNudgeCounter(sessionID string, threshold int) (fire bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	ts := now()
	if _, err := tx.Exec(`
		INSERT INTO sessions (project_id, session_id, started_at, ended_at, last_activity_at, nudge_counter)
		VALUES (?, ?, ?, NULL, ?, 0)
		ON CONFLICT (project_id, session_id) DO NOTHING
	`, s.projectID, sessionID, ts, ts); err != nil {
		return false, err
	}

	var counter int
	if err := tx.QueryRow(`SELECT nudge_counter FROM sessions WHERE project_id = ? AND session_id = ?`, s.projectID, sessionID).Scan(&counter); err != nil {
		return false, err
	}
	counter++
	fire = counter >= threshold
	if fire {
		counter = 0
	}
	if _, err := tx.Exec(`UPDATE sessions SET nudge_counter = ? WHERE project_id = ? AND session_id = ?`, counter, s.projectID, sessionID); err != nil {
		return false, err
	}
	return fire, tx.Commit()
}

package session

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store tracks one project's sessions and file activity, fed by Claude Code's hook events, not any live process state.
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
	messaging_socket TEXT,
	messaging_token TEXT,
	thread_read_at TEXT NOT NULL DEFAULT '',
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
CREATE TABLE IF NOT EXISTS thread_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	body TEXT NOT NULL,
	mention TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_thread_messages_project ON thread_messages(project_id, created_at);
CREATE INDEX IF NOT EXISTS idx_thread_messages_mention ON thread_messages(project_id, mention, created_at);
CREATE TABLE IF NOT EXISTS thread_participants (
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	claude_session_id TEXT,
	started_at TEXT NOT NULL,
	ended_at TEXT,
	last_activity_at TEXT NOT NULL,
	PRIMARY KEY (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_thread_participants_open ON thread_participants(project_id, ended_at);
CREATE TABLE IF NOT EXISTS lg_tips (
	project_id TEXT NOT NULL,
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	message TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_lg_tips_project ON lg_tips(project_id);
CREATE TABLE IF NOT EXISTS lg_tab_sessions (
	project_id TEXT NOT NULL,
	tab_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (project_id, tab_id)
);
CREATE TABLE IF NOT EXISTS lg_signatures (
	project_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	checklist_id TEXT NOT NULL,
	signed_at TEXT NOT NULL,
	PRIMARY KEY (project_id, session_id, checklist_id)
);
`

// Open may run before config.Dir() exists, since a hook can fire before any other lgrass call in a project.
func Open(dbPath, projectID string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(dbPath), err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// Every hook in every live session opens this file concurrently; busy_timeout retries instead of erroring, WAL keeps readers from blocking on a writer.
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configuring %s: %w", dbPath, err)
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

// RFC3339Nano, not RFC3339: second-only precision let a Start and an immediately-following PostThreadMessage land on the same string, defeating UnreadMentions' strict ">" check.
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// Resets thread_read_at and clears this session_id's checklist signatures, so a fresh or reused session starts ungated and unflooded by past mentions.
func (s *Store) Start(sessionID, messagingSocket, messagingToken string) error {
	ts := now()
	if _, err := s.db.Exec(`DELETE FROM lg_signatures WHERE project_id = ? AND session_id = ?`, s.projectID, sessionID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO sessions (project_id, session_id, started_at, ended_at, last_activity_at, nudge_counter, messaging_socket, messaging_token, thread_read_at)
		VALUES (?, ?, ?, NULL, ?, 0, ?, ?, ?)
		ON CONFLICT (project_id, session_id) DO UPDATE SET
			started_at = excluded.started_at,
			ended_at = NULL,
			last_activity_at = excluded.last_activity_at,
			nudge_counter = 0,
			messaging_socket = excluded.messaging_socket,
			messaging_token = excluded.messaging_token,
			thread_read_at = excluded.thread_read_at
	`, s.projectID, sessionID, ts, ts, messagingSocket, messagingToken, ts)
	return err
}

func (s *Store) EnsureOpen(sessionID string) error {
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO sessions (project_id, session_id, started_at, ended_at, last_activity_at, nudge_counter, thread_read_at)
		VALUES (?, ?, ?, NULL, ?, 0, ?)
		ON CONFLICT (project_id, session_id) DO UPDATE SET
			ended_at = NULL,
			last_activity_at = excluded.last_activity_at
	`, s.projectID, sessionID, ts, ts, ts)
	return err
}

// Upserts so a missed or misconfigured SessionStart hook doesn't leave liveness permanently blind to this session.
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

func (s *Store) End(sessionID string) error {
	_, err := s.db.Exec(`UPDATE sessions SET ended_at = ? WHERE project_id = ? AND session_id = ?`, now(), s.projectID, sessionID)
	return err
}

func (s *Store) LogFileActivity(sessionID, filePath string) error {
	_, err := s.db.Exec(`
		INSERT INTO file_activity (project_id, session_id, file_path, dir, touched_at)
		VALUES (?, ?, ?, ?, ?)
	`, s.projectID, sessionID, filePath, filepath.Dir(filePath), now())
	return err
}

type ActivityHit struct {
	SessionID string
	FilePath  string
	TouchedAt string
	SameFile  bool // false means a sibling file in the same directory, not the same file
}

// Deduplicated to one hit per session, most recent first.
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

type SessionStatus struct {
	SessionID string
	Active    bool // false means idling: no tool activity within the threshold
}

// len(result) is also the population count: how many other sessions are open right now.
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

// Upserts for the same reason Touch does.
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

type ThreadMessage struct {
	ID        int64
	SessionID string // author
	Body      string
	Mention   string // target session_id, "" when not aimed at anyone specific
	CreatedAt string
}

// No separate thread/topic object exists: the project itself is the channel.
func (s *Store) PostThreadMessage(sessionID, body, mention string) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO thread_messages (project_id, session_id, body, mention, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, s.projectID, sessionID, body, mention, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RecentThreadMessages(limit int) ([]ThreadMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, body, mention, created_at FROM thread_messages
		WHERE project_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, s.projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadMessages(rows)
}

// The pull-based fallback for delivery, surfaced via hook additionalContext, when the live socket push in deliver.go fails or the target has no socket at all.
func (s *Store) UnreadMentions(sessionID string) ([]ThreadMessage, error) {
	var readAt string
	if err := s.db.QueryRow(`SELECT thread_read_at FROM sessions WHERE project_id = ? AND session_id = ?`, s.projectID, sessionID).Scan(&readAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, session_id, body, mention, created_at FROM thread_messages
		WHERE project_id = ? AND mention = ? AND session_id != ? AND created_at > ?
		ORDER BY created_at ASC, id ASC
	`, s.projectID, sessionID, sessionID, readAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadMessages(rows)
}

func (s *Store) MarkThreadRead(sessionID string) error {
	_, err := s.db.Exec(`UPDATE sessions SET thread_read_at = ? WHERE project_id = ? AND session_id = ?`, now(), s.projectID, sessionID)
	return err
}

func scanThreadMessages(rows *sql.Rows) ([]ThreadMessage, error) {
	var out []ThreadMessage
	for rows.Next() {
		var m ThreadMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Body, &m.Mention, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// `thread listen` uses this as its polling cursor, so it only ever surfaces messages posted after it started, never replays history (`thread list` covers that).
func (s *Store) LatestThreadMessageID() (int64, error) {
	var id sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(id) FROM thread_messages WHERE project_id = ?`, s.projectID).Scan(&id); err != nil {
		return 0, err
	}
	return id.Int64, nil
}

// A message-id cursor, not a timestamp one, so listen's poll loop can't hit the same collision class RFC3339Nano exists to avoid.
func (s *Store) NewThreadMessagesAfter(afterID int64) ([]ThreadMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, body, mention, created_at FROM thread_messages
		WHERE project_id = ? AND id > ?
		ORDER BY id ASC
	`, s.projectID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadMessages(rows)
}

type MessagingTarget struct {
	SessionID string
	Socket    string
	Token     string
}

func (s *Store) LiveMessagingTargets(excludeSessionID string) ([]MessagingTarget, error) {
	rows, err := s.db.Query(`
		SELECT session_id, messaging_socket, messaging_token FROM sessions
		WHERE project_id = ? AND ended_at IS NULL AND session_id != ?
			AND messaging_socket IS NOT NULL AND messaging_socket != ''
	`, s.projectID, excludeSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MessagingTarget
	for rows.Next() {
		var t MessagingTarget
		if err := rows.Scan(&t.SessionID, &t.Socket, &t.Token); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Participant is the lgrass-owned addressable identity for thread commands, independent of the hook-driven sessions table.
type Participant struct {
	Name            string
	ClaudeSessionID string
}

func (s *Store) HasOpenSession(sessionID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`
		SELECT 1 FROM sessions WHERE project_id = ? AND session_id = ? AND ended_at IS NULL
	`, s.projectID, sessionID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return exists == 1, err
}

// Appends -2, -3, ... on collision with another currently-live participant.
func (s *Store) BeginParticipant(name, claudeSessionID string) (string, error) {
	assigned := name
	for suffix := 2; ; suffix++ {
		var exists int
		err := s.db.QueryRow(`
			SELECT 1 FROM thread_participants WHERE project_id = ? AND name = ? AND ended_at IS NULL
		`, s.projectID, assigned).Scan(&exists)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return "", err
		}
		assigned = fmt.Sprintf("%s-%d", name, suffix)
	}

	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO thread_participants (project_id, name, claude_session_id, started_at, ended_at, last_activity_at)
		VALUES (?, ?, ?, ?, NULL, ?)
		ON CONFLICT (project_id, name) DO UPDATE SET
			claude_session_id = excluded.claude_session_id,
			started_at = excluded.started_at,
			ended_at = NULL,
			last_activity_at = excluded.last_activity_at
	`, s.projectID, assigned, nullableString(claudeSessionID), ts, ts)
	if err != nil {
		return "", err
	}
	return assigned, nil
}

func (s *Store) EndParticipant(name string) error {
	_, err := s.db.Exec(`UPDATE thread_participants SET ended_at = ? WHERE project_id = ? AND name = ?`, now(), s.projectID, name)
	return err
}

// Zero-value ClaudeSessionID, no error, when name isn't a currently-live participant.
func (s *Store) ParticipantByName(name string) (Participant, error) {
	p := Participant{Name: name}
	var claudeID sql.NullString
	err := s.db.QueryRow(`
		SELECT claude_session_id FROM thread_participants WHERE project_id = ? AND name = ? AND ended_at IS NULL
	`, s.projectID, name).Scan(&claudeID)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if claudeID.Valid {
		p.ClaudeSessionID = claudeID.String
	}
	return p, nil
}

func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

type Tip struct {
	ID      int64
	Message string
}

func (s *Store) AddTip(message string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO lg_tips (project_id, message, created_at) VALUES (?, ?, ?)`, s.projectID, message, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListTips() ([]Tip, error) {
	rows, err := s.db.Query(`SELECT id, message FROM lg_tips WHERE project_id = ? ORDER BY id`, s.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Tip
	for rows.Next() {
		var t Tip
		if err := rows.Scan(&t.ID, &t.Message); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteTip(id int64) error {
	_, err := s.db.Exec(`DELETE FROM lg_tips WHERE project_id = ? AND id = ?`, s.projectID, id)
	return err
}

// Sign upserts, so re-signing before the previous TTL expires just resets the clock.
func (s *Store) Sign(sessionID, checklistID string) error {
	_, err := s.db.Exec(`
		INSERT INTO lg_signatures (project_id, session_id, checklist_id, signed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (project_id, session_id, checklist_id) DO UPDATE SET signed_at = excluded.signed_at
	`, s.projectID, sessionID, checklistID, now())
	return err
}

// SignedAt returns the zero time, no error, when this session has never signed this checklist.
func (s *Store) SignedAt(sessionID, checklistID string) (time.Time, error) {
	var ts string
	err := s.db.QueryRow(`
		SELECT signed_at FROM lg_signatures WHERE project_id = ? AND session_id = ? AND checklist_id = ?
	`, s.projectID, sessionID, checklistID).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, ts)
}

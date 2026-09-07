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
`

// Open opens (creating and migrating if needed) the shared session
// database at dbPath, scoped to one project. This is very often the
// first lgrass call in a project at all, since a hook fires before any
// knowledge command ever runs, so it can't assume config.Dir() exists
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

// RFC3339Nano, not RFC3339: thread_read_at and thread_messages.created_at
// get compared with a strict ">" (see UnreadMentions), and second-only
// precision let a Start and an immediately-following PostThreadMessage
// land on the same string, silently hiding the message from the > check.
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// Start records a session beginning, or restarting under a reused id
// (ended_at and nudge_counter reset either way). messagingSocket/
// messagingToken are this session's own Claude Code inbox socket
// (CLAUDE_CODE_MESSAGING_SOCKET/_TOKEN from its own environment, empty
// when messaging isn't available); other sessions read these back to
// push thread messages directly into it. thread_read_at resets to now,
// so a fresh or restarted session doesn't get flooded with mentions
// posted before it existed; `thread list` is there to catch up on
// history deliberately.
func (s *Store) Start(sessionID, messagingSocket, messagingToken string) error {
	ts := now()
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

// ThreadMessage is one project-wide thread post.
type ThreadMessage struct {
	ID        int64
	SessionID string // author
	Body      string
	Mention   string // target session_id, "" when not aimed at anyone specific
	CreatedAt string
}

// PostThreadMessage appends a message to the project's shared thread log
// and returns its id. There is no separate thread/topic object to create
// first: the project itself is the channel, per the project-wide (not
// point-to-point) thread model.
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

// RecentThreadMessages returns the project's most recent thread messages,
// newest first, for a pane catching up cold via `thread list`.
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

// UnreadMentions returns messages that mention sessionID and arrived
// since it last checked (its thread_read_at), oldest first. This is the
// pull-based fallback surfaced via hook additionalContext, guaranteeing
// delivery even when the live socket push in deliver.go fails or the
// target session has no messaging socket at all.
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

// MarkThreadRead advances sessionID's thread_read_at to now, so the same
// mention isn't surfaced again on the next hook firing.
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

// LatestThreadMessageID returns the highest thread_messages id in the
// project so far, 0 when there are none yet. `lgrass thread listen`
// calls this once at startup as its polling cursor, so it only ever
// surfaces messages posted after it started listening, never replays
// history. `thread list` is there for that.
func (s *Store) LatestThreadMessageID() (int64, error) {
	var id sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(id) FROM thread_messages WHERE project_id = ?`, s.projectID).Scan(&id); err != nil {
		return 0, err
	}
	return id.Int64, nil
}

// NewThreadMessagesAfter returns every project message with an id
// greater than afterID, oldest first. A message-id cursor, not a
// timestamp one, so listen's poll loop can't hit the same
// same-timestamp-collision class of bug UnreadMentions needed
// RFC3339Nano to avoid.
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

// MessagingTarget is another live session's own Claude Code inbox
// socket, for pushing a thread message directly into it.
type MessagingTarget struct {
	SessionID string
	Socket    string
	Token     string
}

// LiveMessagingTargets returns every other currently-live session in the
// project that captured a messaging socket at SessionStart.
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

// Participant is one thread_participants row, the addressable identity for thread commands.
type Participant struct {
	Name            string
	ClaudeSessionID string
}

// HasOpenSession reports whether sessionID has a live (not yet ended)
// row in the hook-driven sessions table.
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

// BeginParticipant registers name as a live participant, appending -2, -3, ... on collision, and returns the name actually assigned.
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

// EndParticipant marks name as no longer live.
func (s *Store) EndParticipant(name string) error {
	_, err := s.db.Exec(`UPDATE thread_participants SET ended_at = ? WHERE project_id = ? AND name = ?`, now(), s.projectID, name)
	return err
}

// ParticipantByName looks up name's live thread_participants row, zero-value if not found.
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

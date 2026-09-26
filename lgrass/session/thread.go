package session

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	MaxMessageRunes = 2000
	MaxTitleRunes   = 120
)

var ErrNoSuchThread = errors.New("session: no such thread in this project")

type Thread struct {
	ID             int64
	Title          string
	GroupID        int64 // 0 for an ad hoc thread
	CreatedBy      string
	CreatedAt      string
	LastActivityAt string
	MessageCount   int
}

type Message struct {
	ID        int64
	ThreadID  int64
	TabID     string
	Body      string
	CreatedAt string
	Mentions  []string
}

func validateMessage(body string) error {
	if strings.TrimSpace(body) == "" {
		return errors.New("session: message is empty")
	}
	if n := utf8.RuneCountInString(body); n > MaxMessageRunes {
		return fmt.Errorf("session: message is %d characters and the cap is %d, put the long content in a scratchpad note and post a pointer to it", n, MaxMessageRunes)
	}
	return nil
}

func validateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("session: thread title is empty")
	}
	if n := utf8.RuneCountInString(title); n > MaxTitleRunes {
		return fmt.Errorf("session: thread title is %d characters and the cap is %d", n, MaxTitleRunes)
	}
	return nil
}

// Opens an ad hoc thread, whose first message is content.
func (s *Store) CreateThread(tabID, title, content string) (int64, error) {
	title = strings.TrimSpace(title)
	if err := validateTitle(title); err != nil {
		return 0, err
	}
	if err := validateMessage(content); err != nil {
		return 0, err
	}
	mentions, err := s.ResolveMentions(ParseMentions(content))
	if err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	ts := now()
	res, err := tx.Exec(`INSERT INTO lg_threads (project_id, title, created_by, created_at) VALUES (?, ?, ?, ?)`, s.projectID, title, tabID, ts)
	if err != nil {
		return 0, err
	}
	threadID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := insertMessage(tx, threadID, tabID, content, mentions, ts); err != nil {
		return 0, err
	}
	return threadID, tx.Commit()
}

func (s *Store) PostMessage(tabID string, threadID int64, content string) (Message, error) {
	if err := validateMessage(content); err != nil {
		return Message{}, err
	}
	if _, err := s.ThreadByID(threadID); err != nil {
		return Message{}, err
	}
	mentions, err := s.ResolveMentions(ParseMentions(content))
	if err != nil {
		return Message{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()

	msg, err := insertMessage(tx, threadID, tabID, content, mentions, now())
	if err != nil {
		return Message{}, err
	}
	return msg, tx.Commit()
}

func insertMessage(tx *sql.Tx, threadID int64, tabID, body string, mentions []string, ts string) (Message, error) {
	res, err := tx.Exec(`INSERT INTO lg_messages (thread_id, tab_id, body, created_at) VALUES (?, ?, ?, ?)`, threadID, tabID, body, ts)
	if err != nil {
		return Message{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	for _, tab := range mentions {
		if _, err := tx.Exec(`INSERT INTO lg_message_mentions (message_id, tab_id) VALUES (?, ?)`, id, tab); err != nil {
			return Message{}, err
		}
	}
	return Message{ID: id, ThreadID: threadID, TabID: tabID, Body: body, CreatedAt: ts, Mentions: mentions}, nil
}

func (s *Store) ThreadByID(id int64) (Thread, error) {
	var t Thread
	var group sql.NullInt64
	err := s.db.QueryRow(`
		SELECT t.id, t.title, t.group_id, t.created_by, t.created_at,
			COALESCE((SELECT created_at FROM lg_messages WHERE thread_id = t.id ORDER BY id DESC LIMIT 1), t.created_at),
			(SELECT COUNT(*) FROM lg_messages WHERE thread_id = t.id)
		FROM lg_threads t WHERE t.project_id = ? AND t.id = ?
	`, s.projectID, id).Scan(&t.ID, &t.Title, &group, &t.CreatedBy, &t.CreatedAt, &t.LastActivityAt, &t.MessageCount)
	if err == sql.ErrNoRows {
		return Thread{}, ErrNoSuchThread
	}
	if err != nil {
		return Thread{}, err
	}
	t.GroupID = group.Int64
	return t, nil
}

// Newest first, at most limit messages older than beforeID (0 for the newest page); more reports whether older ones remain.
func (s *Store) ReadThread(threadID, beforeID int64, limit int) (msgs []Message, more bool, err error) {
	if limit < 1 {
		limit = 1
	}
	query := `SELECT id, thread_id, tab_id, body, created_at FROM lg_messages WHERE thread_id = ?`
	args := []interface{}{threadID}
	if beforeID > 0 {
		query += ` AND id < ?`
		args = append(args, beforeID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.TabID, &m.Body, &m.CreatedAt); err != nil {
			return nil, false, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(msgs) > limit {
		msgs = msgs[:limit]
		more = true
	}
	for i := range msgs {
		if msgs[i].Mentions, err = s.messageMentions(msgs[i].ID); err != nil {
			return nil, false, err
		}
	}
	return msgs, more, nil
}

func (s *Store) messageMentions(messageID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT tab_id FROM lg_message_mentions WHERE message_id = ? ORDER BY tab_id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Most recently active first.
func (s *Store) ListThreads(limit int) ([]Thread, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.title, t.group_id, t.created_by, t.created_at,
			COALESCE(MAX(m.created_at), t.created_at), COUNT(m.id)
		FROM lg_threads t LEFT JOIN lg_messages m ON m.thread_id = t.id
		WHERE t.project_id = ?
		GROUP BY t.id
		ORDER BY COALESCE(MAX(m.id), 0) DESC, t.id DESC
		LIMIT ?
	`, s.projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Thread
	for rows.Next() {
		var t Thread
		var group sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Title, &group, &t.CreatedBy, &t.CreatedAt, &t.LastActivityAt, &t.MessageCount); err != nil {
			return nil, err
		}
		t.GroupID = group.Int64
		out = append(out, t)
	}
	return out, rows.Err()
}

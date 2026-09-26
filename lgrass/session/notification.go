package session

import (
	"database/sql"
	"strings"
	"time"
)

// Kept after being read so the ledger doubles as an audit trail.
const NotificationRetention = 30 * 24 * time.Hour

const (
	stateSent = "sent"
	stateRead = "read"

	// A listener that has not written within this window no longer counts as live.
	listenerHeartbeatTTL = time.Hour
)

// The pending notifications of one tab for one thread.
type PendingThread struct {
	ThreadID int64
	Title    string
	Senders  []string // distinct author tab ids, in order of first appearance
	RowIDs   []int64
}

func (p PendingThread) Count() int { return len(p.RowIDs) }

// A tab id is globally unique, so this ignores the store's project.
func (s *Store) PendingForTab(tabID string) ([]PendingThread, error) {
	rows, err := s.db.Query(`
		SELECT n.id, n.thread_id, t.title, m.tab_id
		FROM lg_notifications n
		JOIN lg_threads t ON t.id = n.thread_id
		JOIN lg_messages m ON m.id = n.message_id
		WHERE n.target_tab_id = ? AND n.state = 'pending'
		ORDER BY n.id
	`, tabID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingThread
	index := map[int64]int{}
	for rows.Next() {
		var rowID, threadID int64
		var title, author string
		if err := rows.Scan(&rowID, &threadID, &title, &author); err != nil {
			return nil, err
		}
		i, ok := index[threadID]
		if !ok {
			i = len(out)
			index[threadID] = i
			out = append(out, PendingThread{ThreadID: threadID, Title: title})
		}
		out[i].RowIDs = append(out[i].RowIDs, rowID)
		if !containsString(out[i].Senders, author) {
			out[i].Senders = append(out[i].Senders, author)
		}
	}
	return out, rows.Err()
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func idPlaceholders(ids []int64) (string, []interface{}) {
	marks := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args[i] = id
	}
	return strings.Join(marks, ","), args
}

func (s *Store) MarkNotificationsSent(rowIDs []int64) error {
	if len(rowIDs) == 0 {
		return nil
	}
	marks, args := idPlaceholders(rowIDs)
	_, err := s.db.Exec(`UPDATE lg_notifications SET state = 'sent', sent_at = ?, attempts = attempts + 1, last_attempt_at = ? WHERE state = 'pending' AND id IN (`+marks+`)`, append([]interface{}{now(), now()}, args...)...)
	return err
}

func (s *Store) RecordNotificationAttempt(rowIDs []int64) error {
	if len(rowIDs) == 0 {
		return nil
	}
	marks, args := idPlaceholders(rowIDs)
	_, err := s.db.Exec(`UPDATE lg_notifications SET attempts = attempts + 1, last_attempt_at = ? WHERE id IN (`+marks+`)`, append([]interface{}{now()}, args...)...)
	return err
}

// Settles the tab's rows for exactly the messages it read.
func (s *Store) MarkMessagesRead(tabID string, messageIDs []int64) error {
	if len(messageIDs) == 0 {
		return nil
	}
	marks, args := idPlaceholders(messageIDs)
	_, err := s.db.Exec(`UPDATE lg_notifications SET state = 'read', read_at = ? WHERE target_tab_id = ? AND state != 'read' AND message_id IN (`+marks+`)`, append([]interface{}{now(), tabID}, args...)...)
	return err
}

// Tabs that have pending rows, whatever their vendor.
func (s *Store) PendingTabs() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT target_tab_id FROM lg_notifications WHERE state = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tab string
		if err := rows.Scan(&tab); err != nil {
			return nil, err
		}
		out = append(out, tab)
	}
	return out, rows.Err()
}

// Tabs with a pending row that has attempts left and whose backoff since the last attempt has elapsed.
func (s *Store) TabsDueForRetry(maxAttempts int, backoff func(attempts int) time.Duration, at time.Time) ([]string, error) {
	rows, err := s.db.Query(`SELECT target_tab_id, attempts, last_attempt_at FROM lg_notifications WHERE state = 'pending' AND attempts < ?`, maxAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var tab string
		var attempts int
		var last sql.NullString
		if err := rows.Scan(&tab, &attempts, &last); err != nil {
			return nil, err
		}
		if seen[tab] {
			continue
		}
		due := true
		if last.Valid && attempts > 0 {
			if t, err := time.Parse(time.RFC3339Nano, last.String); err == nil {
				due = !at.Before(t.Add(backoff(attempts)))
			}
		}
		if due {
			seen[tab] = true
			out = append(out, tab)
		}
	}
	return out, rows.Err()
}

func (s *Store) PruneNotifications(at time.Time) error {
	cutoff := at.Add(-NotificationRetention).UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`DELETE FROM lg_notifications WHERE created_at < ?`, cutoff)
	return err
}

// Also drops every heartbeat older than the TTL, so tabs that never come back leave no rows.
func (s *Store) Heartbeat(tabID string) error {
	ts := now()
	if _, err := s.db.Exec(`
		INSERT INTO lg_listener_heartbeats (project_id, tab_id, seen_at) VALUES (?, ?, ?)
		ON CONFLICT (project_id, tab_id) DO UPDATE SET seen_at = excluded.seen_at
	`, s.projectID, tabID, ts); err != nil {
		return err
	}
	cutoff := time.Now().Add(-listenerHeartbeatTTL).UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`DELETE FROM lg_listener_heartbeats WHERE seen_at < ?`, cutoff)
	return err
}

// Empty string, no error, when neither a tab record nor a group membership names the tab.
func (s *Store) VendorForTab(tabID string) (string, error) {
	var vendor string
	err := s.db.QueryRow(`SELECT vendor FROM lg_tabs WHERE tab_id = ? LIMIT 1`, tabID).Scan(&vendor)
	if err == nil {
		return vendor, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	err = s.db.QueryRow(`SELECT vendor FROM lg_group_members WHERE tab_id = ? ORDER BY group_id DESC LIMIT 1`, tabID).Scan(&vendor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return vendor, err
}

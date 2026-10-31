// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// HumanAuthor stands in for a tab id on messages the human posts from the app into a conflict thread. Tab ids are UUIDs, so it never collides with a tab.
const HumanAuthor = "human"

const conflictScopePrefix = "conflict:"

var ErrNoConflict = errors.New("session: no conflict is handed to a tab in this project")

// NotAssigneeError refuses a conflict command from a tab the human did not hand the conflict to.
type NotAssigneeError struct{ Assignee string }

func (e NotAssigneeError) Error() string {
	return fmt.Sprintf("session: the conflict is handed to %s, not this tab", TabLabel(e.Assignee))
}

// HumanMessage is one message the human wrote in the app's conflict thread, with an optional line anchor.
type HumanMessage struct {
	Body   string
	Anchor *Anchor
}

func validateScope(scope string) error {
	if !strings.HasPrefix(scope, conflictScopePrefix) || len(scope) == len(conflictScopePrefix) {
		return fmt.Errorf("session: %q is not a conflict scope", scope)
	}
	return nil
}

func validateAnchor(a *Anchor) error {
	if a == nil {
		return nil
	}
	if strings.TrimSpace(a.Path) == "" || a.StartLine < 1 || a.EndLine < a.StartLine {
		return fmt.Errorf("session: anchor %s %d-%d is not a path with lines N-M, N at least 1", a.Path, a.StartLine, a.EndLine)
	}
	return nil
}

func conflictTitle(scope string) string {
	title := strings.TrimPrefix(scope, conflictScopePrefix)
	for utf8.RuneCountInString(title) > MaxTitleRunes {
		_, size := utf8.DecodeLastRuneInString(title)
		title = title[:len(title)-size]
	}
	return title
}

// OpenConflict opens the project's conflict thread for scope and hands it to assignee, reusing the open thread when it already has that scope. An open thread for another scope is closed first, since the conflict it was about has ended. baseline is what the conflicted files held at hand-off, which submit compares against.
func (s *Store) OpenConflict(scope, assignee, baseline string) (Thread, error) {
	if err := validateScope(scope); err != nil {
		return Thread{}, err
	}
	vendor, err := s.TabVendor(assignee)
	if err != nil {
		return Thread{}, err
	}
	if vendor != "claude" && vendor != "codex" {
		return Thread{}, fmt.Errorf("session: tab %s is not an open Claude or Codex tab in this project", TabLabel(assignee))
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Thread{}, err
	}
	defer tx.Rollback()

	ts := now()
	var id, openScope string
	err = tx.QueryRow(`SELECT id, scope FROM lg_threads WHERE project_id = ? AND kind = ? AND closed_at IS NULL`, s.projectID, ThreadConflict).Scan(&id, &openScope)
	switch {
	case err == sql.ErrNoRows:
		id = ""
	case err != nil:
		return Thread{}, err
	case openScope != scope:
		if _, err := tx.Exec(`UPDATE lg_threads SET closed_at = ? WHERE id = ?`, ts, id); err != nil {
			return Thread{}, err
		}
		id = ""
	}
	if id == "" {
		if id, err = newThreadID(tx); err != nil {
			return Thread{}, err
		}
		if _, err := tx.Exec(`INSERT INTO lg_threads (id, project_id, title, created_by, created_at, kind, scope, assignee, baseline) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, s.projectID, conflictTitle(scope), HumanAuthor, ts, ThreadConflict, scope, assignee, baseline); err != nil {
			return Thread{}, err
		}
	} else if _, err := tx.Exec(`UPDATE lg_threads SET assignee = ?, baseline = ? WHERE id = ?`, assignee, baseline, id); err != nil {
		return Thread{}, err
	}
	if err := tx.Commit(); err != nil {
		return Thread{}, err
	}
	return s.ThreadByID(id)
}

// OpenConflictThread returns the project's open conflict thread, or ErrNoConflict.
func (s *Store) OpenConflictThread() (Thread, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM lg_threads WHERE project_id = ? AND kind = ? AND closed_at IS NULL`, s.projectID, ThreadConflict).Scan(&id)
	if err == sql.ErrNoRows {
		return Thread{}, ErrNoConflict
	}
	if err != nil {
		return Thread{}, err
	}
	return s.ThreadByID(id)
}

// ConflictThreadFor returns the open conflict thread when tabID is its assignee, and refuses every other tab.
func (s *Store) ConflictThreadFor(tabID string) (Thread, error) {
	t, err := s.OpenConflictThread()
	if err != nil {
		return Thread{}, err
	}
	if tabID == "" || t.Assignee != tabID {
		return Thread{}, NotAssigneeError{Assignee: t.Assignee}
	}
	return t, nil
}

func (s *Store) ConflictBaseline(threadID string) (string, error) {
	var baseline string
	err := s.db.QueryRow(`SELECT baseline FROM lg_threads WHERE project_id = ? AND id = ?`, s.projectID, threadID).Scan(&baseline)
	if err == sql.ErrNoRows {
		return "", ErrNoSuchThread
	}
	return baseline, err
}

// CloseConflict closes the open conflict thread when its scope is scope and, when threadID is set, its id is threadID. It reports whether a thread was closed; a mismatch closes nothing, since the thread it was meant for has already gone.
func (s *Store) CloseConflict(scope, threadID string) (bool, error) {
	query := `UPDATE lg_threads SET closed_at = ? WHERE project_id = ? AND kind = ? AND closed_at IS NULL AND scope = ?`
	args := []any{now(), s.projectID, ThreadConflict, scope}
	if threadID != "" {
		query += ` AND id = ?`
		args = append(args, threadID)
	}
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// PostHumanMessages posts the human's messages into the open conflict thread for scope, in order. Each one mentions the assignee, so the nudge wakes it.
func (s *Store) PostHumanMessages(scope string, msgs []HumanMessage) ([]Message, error) {
	if len(msgs) == 0 {
		return nil, errors.New("session: no messages to post")
	}
	for _, m := range msgs {
		if err := validateMessage(m.Body); err != nil {
			return nil, err
		}
		if err := validateAnchor(m.Anchor); err != nil {
			return nil, err
		}
	}
	thread, err := s.OpenConflictThread()
	if err != nil {
		return nil, err
	}
	if thread.Scope != scope {
		return nil, fmt.Errorf("session: the open conflict thread is for %s, not %s", thread.Scope, scope)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	assignee := []string{thread.Assignee}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		msg, err := insertMessage(tx, s.projectID, thread.ID, HumanAuthor, m.Body, assignee, assignee, messageMeta{anchor: m.Anchor}, now())
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, tx.Commit()
}

// PostNote posts the assignee's explanation of the lines at anchor. It wakes nobody, since the human reads the thread in the resolver.
func (s *Store) PostNote(tabID string, anchor Anchor, body string) (Message, error) {
	if err := validateAnchor(&anchor); err != nil {
		return Message{}, err
	}
	return s.postConflictMeta(tabID, body, messageMeta{kind: MessageNote, anchor: &anchor})
}

// PostSubmit records the assignee's submission with the conflicted files it changed. It wakes nobody; the app opens the resolver instead.
func (s *Store) PostSubmit(tabID string, body string, files []string) (Message, error) {
	if len(files) == 0 {
		return Message{}, errors.New("session: a submission names at least one file")
	}
	return s.postConflictMeta(tabID, body, messageMeta{kind: MessageSubmit, files: files})
}

func (s *Store) postConflictMeta(tabID, body string, meta messageMeta) (Message, error) {
	if err := validateMessage(body); err != nil {
		return Message{}, err
	}
	thread, err := s.ConflictThreadFor(tabID)
	if err != nil {
		return Message{}, err
	}
	return s.insertConflictMessage(tabID, thread.ID, body, meta)
}

// A plain post into a conflict thread: only from its assignee while it is open, and with no mention, since the thread is where the model talks to the human.
func (s *Store) postToConflict(tabID string, thread Thread, body string) (Message, error) {
	if thread.Closed() {
		return Message{}, fmt.Errorf("session: conflict thread %s is closed", thread.ID)
	}
	if tabID != thread.Assignee {
		return Message{}, NotAssigneeError{Assignee: thread.Assignee}
	}
	if strings.Contains(body, "!>>") {
		return Message{}, errors.New("session: a conflict thread is for talking to the human, so a message there mentions no tab")
	}
	return s.insertConflictMessage(tabID, thread.ID, body, messageMeta{})
}

func (s *Store) insertConflictMessage(tabID, threadID, body string, meta messageMeta) (Message, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	msg, err := insertMessage(tx, s.projectID, threadID, tabID, body, nil, nil, meta, now())
	if err != nil {
		return Message{}, err
	}
	return msg, tx.Commit()
}

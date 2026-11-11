// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"encoding/json"
	"fmt"
	"time"
)

// What happened to the uncommitted changes when the git panel moved the branch.
const (
	CarryNone     = "none"     // the tree was clean
	CarryCarried  = "carried"  // the changes were stashed and applied on the new branch
	CarryConflict = "conflict" // the apply conflicted and the stash was kept
)

// Only tabs with a prompt or tool call this recently are told. Nothing wakes a tab for a branch move, so a longer window only means a tab used earlier today reads the line with its next prompt.
const BranchNoticeWindow = 12 * time.Hour

// The ref of a branch row.
type BranchChange struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Carry string `json:"carry"`
}

func ValidCarry(carry string) bool {
	return carry == CarryNone || carry == CarryCarried || carry == CarryConflict
}

// Inserts one branch row for each Claude, Codex or Antigravity tab of the project with real activity within the window, and returns those tabs. The rows reach a tab only as hook context, never typed (see PendingForTab). The tab handed the open conflict is skipped, since the resolver already reaches it by typed nudge.
func (s *Store) InsertBranchChanged(change BranchChange, at time.Time) ([]string, error) {
	if !ValidCarry(change.Carry) {
		return nil, fmt.Errorf("session: unknown carry %q, want none, carried or conflict", change.Carry)
	}
	ref, err := json.Marshal(change)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	cutoff := at.Add(-BranchNoticeWindow).UTC().Format(time.RFC3339Nano)
	rows, err := tx.Query(`
		SELECT t.tab_id FROM lg_tabs t JOIN lg_tab_state st ON st.tab_id = t.tab_id
		WHERE t.project_id = ? AND t.vendor IN ('claude', 'codex', 'agy') AND st.last_active_at > ?
			AND t.tab_id NOT IN (SELECT assignee FROM lg_threads WHERE project_id = ? AND kind = ? AND closed_at IS NULL)
		ORDER BY t.tab_id
	`, s.projectID, cutoff, s.projectID, ThreadConflict)
	if err != nil {
		return nil, err
	}
	var tabs []string
	for rows.Next() {
		var tab string
		if err := rows.Scan(&tab); err != nil {
			rows.Close()
			return nil, err
		}
		tabs = append(tabs, tab)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ts := now()
	for _, tab := range tabs {
		if _, err := tx.Exec(`
			INSERT INTO lg_notifications (project_id, source, ref, target_tab_id, kind, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, s.projectID, SourceBranch, string(ref), tab, KindYou, ts); err != nil {
			return nil, err
		}
	}
	return tabs, tx.Commit()
}

// Worse carries win when moves are combined: a conflict outranks carried changes, which outrank a clean tree.
func carryRank(carry string) int {
	switch carry {
	case CarryConflict:
		return 2
	case CarryCarried:
		return 1
	}
	return 0
}

// Several moves before the tab's next prompt or tool call read as one, the same rule gitNotice.ts applies to a burst: the first from, the last to, the worst carry. Moves that end where they started say nothing unless one left a conflict, since the tree is where the tab last saw it. False when there is nothing to say.
func combinedBranchLine(events []PendingEvent) (string, bool) {
	var combined BranchChange
	found := false
	for _, e := range events {
		if e.Source != SourceBranch {
			continue
		}
		var c BranchChange
		if err := json.Unmarshal([]byte(e.Ref), &c); err != nil {
			return Prefix + " the git branch changed. Check git status before you continue.", true
		}
		if !found {
			combined = c
			found = true
			continue
		}
		combined.To = c.To
		if carryRank(c.Carry) > carryRank(combined.Carry) {
			combined.Carry = c.Carry
		}
	}
	if !found || (combined.From == combined.To && combined.Carry != CarryConflict) {
		return "", false
	}
	return formatBranch(combined), true
}

// Composed from a fixed template, never from model text.
func formatBranchChange(ref string) string {
	var c BranchChange
	if err := json.Unmarshal([]byte(ref), &c); err != nil {
		return Prefix + " the git branch changed. Check git status before you continue."
	}
	return formatBranch(c)
}

// Branch names pass git's ref format check, so they hold no spaces or control characters. A conflict line says the human owns it, because a tab that was working when the conflict appeared tends to take it for its own and resolve it.
func formatBranch(c BranchChange) string {
	switch c.Carry {
	case CarryCarried:
		return fmt.Sprintf("%s branch changed from %s to %s, uncommitted changes carried over. Check git status before you continue.", Prefix, c.From, c.To)
	case CarryConflict:
		return fmt.Sprintf("%s the human moved the branch from %s to %s and the carried changes conflicted, so the stash was kept. The human is resolving this conflict. Do not edit the conflicted files or run stash, merge or rebase commands unless the conflict is handed to you.", Prefix, c.From, c.To)
	}
	return fmt.Sprintf("%s branch changed from %s to %s. Check git status before you continue.", Prefix, c.From, c.To)
}

// One line per event row, after the thread lines.
func formatEvent(e PendingEvent) string {
	switch e.Source {
	case SourceBranch:
		return formatBranchChange(e.Ref)
	case SourceAsk:
		return formatAskAnswered(e.Ref)
	}
	return fmt.Sprintf("%s a %s notification arrived.", Prefix, e.Source)
}

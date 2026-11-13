// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// A block is the Nth conflict, 1-based, of a file in git's merge of its stages, which stays the same however the working file is edited.

const (
	VerdictAccepted = "accepted"
	VerdictRejected = "rejected"
	VerdictResolved = "resolved" // by the human, in the three panes
	VerdictAnswered = "answered"
	VerdictAsked    = "asked"

	maxOptions      = 6
	maxOptionRunes  = 200
	maxVerdicts     = 1000
	maxDigestLength = 64
)

// BlockReport is the structured part of a model's decision or question on one block.
type BlockReport struct {
	Path    string   `json:"path"`
	Block   int      `json:"block"`
	Options []string `json:"options,omitempty"` // a question's choices, in order
}

// Verdict is the human's word on one block. Text is the sentence of a rejection, an answer or a question the human asks; Question is the id of the model's question an answer answers; Digest identifies the block's lines an accepted or resolved verdict settled, so a later change to them shows.
type Verdict struct {
	Block    int    `json:"block"`
	Verdict  string `json:"verdict"`
	Text     string `json:"text,omitempty"`
	Question int64  `json:"question,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

// Verdicts is the structured part of a verdicts message: the human's verdicts on blocks of one file.
type Verdicts struct {
	Path     string    `json:"path"`
	Verdicts []Verdict `json:"verdicts"`
}

func validateBlock(path string, block int) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("session: a block report names a file")
	}
	if block < 1 {
		return fmt.Errorf("session: block %d is not a block number, blocks count from 1", block)
	}
	return nil
}

// PostDecision records that the assignee wrote its resolution of a block into the working file, with why. It wakes nobody, since the human reads decisions in the resolver.
func (s *Store) PostDecision(tabID, path string, block int, why string) (Message, error) {
	if err := validateBlock(path, block); err != nil {
		return Message{}, err
	}
	data, err := json.Marshal(BlockReport{Path: path, Block: block})
	if err != nil {
		return Message{}, err
	}
	return s.postConflictMeta(tabID, why, messageMeta{kind: MessageDecision, data: string(data)})
}

// PostQuestion records the assignee's question about a block it left with its markers, with the options it offers the human.
func (s *Store) PostQuestion(tabID, path string, block int, question string, options []string) (Message, error) {
	if err := validateBlock(path, block); err != nil {
		return Message{}, err
	}
	if len(options) > maxOptions {
		return Message{}, fmt.Errorf("session: a question offers at most %d options", maxOptions)
	}
	clean := make([]string, 0, len(options))
	for _, o := range options {
		o = strings.TrimSpace(o)
		if o == "" {
			return Message{}, errors.New("session: an option is empty")
		}
		if utf8.RuneCountInString(o) > maxOptionRunes {
			return Message{}, fmt.Errorf("session: an option is longer than %d characters; put the detail in the question", maxOptionRunes)
		}
		clean = append(clean, o)
	}
	data, err := json.Marshal(BlockReport{Path: path, Block: block, Options: clean})
	if err != nil {
		return Message{}, err
	}
	return s.postConflictMeta(tabID, question, messageMeta{kind: MessageQuestion, data: string(data)})
}

func validateVerdict(v Verdict) error {
	if v.Block < 1 {
		return fmt.Errorf("session: block %d is not a block number", v.Block)
	}
	switch v.Verdict {
	case VerdictAccepted, VerdictResolved:
		if v.Digest == "" || len(v.Digest) > maxDigestLength {
			return fmt.Errorf("session: an %s verdict on block %d carries the digest of its lines", v.Verdict, v.Block)
		}
	case VerdictRejected, VerdictAnswered, VerdictAsked:
		if strings.TrimSpace(v.Text) == "" {
			return fmt.Errorf("session: the %s verdict on block %d has no text", v.Verdict, v.Block)
		}
		if n := utf8.RuneCountInString(v.Text); n > MaxMessageRunes {
			return fmt.Errorf("session: the text on block %d is %d characters and the cap is %d", v.Block, n, MaxMessageRunes)
		}
	default:
		return fmt.Errorf("session: %q is not a verdict", v.Verdict)
	}
	if v.Verdict == VerdictAnswered && v.Question < 1 {
		return fmt.Errorf("session: the answer on block %d does not say which question it answers", v.Block)
	}
	return nil
}

// PostVerdicts posts the human's verdicts on one file into the open conflict thread for scope, as one message. It mentions the assignee, which wakes it, unless quiet, used for what needs no reply, such as blocks the human resolved before handing the conflict over or a file it is about to stage.
func (s *Store) PostVerdicts(scope string, batch Verdicts, quiet bool) (Message, error) {
	if strings.TrimSpace(batch.Path) == "" {
		return Message{}, errors.New("session: verdicts name a file")
	}
	if len(batch.Verdicts) == 0 || len(batch.Verdicts) > maxVerdicts {
		return Message{}, fmt.Errorf("session: a verdicts batch holds 1 to %d verdicts", maxVerdicts)
	}
	for i := range batch.Verdicts {
		batch.Verdicts[i].Text = strings.TrimSpace(batch.Verdicts[i].Text)
		if err := validateVerdict(batch.Verdicts[i]); err != nil {
			return Message{}, err
		}
	}
	thread, err := s.OpenConflictThread()
	if err != nil {
		return Message{}, err
	}
	if thread.Scope != scope {
		return Message{}, fmt.Errorf("session: the open conflict thread is for %s, not %s", thread.Scope, scope)
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return Message{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback()
	for _, v := range batch.Verdicts {
		if v.Verdict != VerdictAnswered {
			continue
		}
		var kind string
		if err := tx.QueryRow(`SELECT kind FROM lg_messages WHERE id = ? AND thread_id = ?`, v.Question, thread.ID).Scan(&kind); err != nil || kind != MessageQuestion {
			return Message{}, fmt.Errorf("session: message #%d is not a question in conflict thread %s", v.Question, thread.ID)
		}
	}
	var to []string
	if !quiet {
		to = []string{thread.Assignee}
	}
	msg, err := insertMessage(tx, s.projectID, thread.ID, HumanAuthor, verdictsSummary(batch), to, to, messageMeta{kind: MessageVerdicts, data: string(data)}, now())
	if err != nil {
		return Message{}, err
	}
	return msg, tx.Commit()
}

// The stored body: one line a reader without the data can still follow. The full verdicts are printed from the data.
func verdictsSummary(batch Verdicts) string {
	counts := map[string]int{}
	for _, v := range batch.Verdicts {
		counts[v.Verdict]++
	}
	var parts []string
	for _, kind := range []string{VerdictAccepted, VerdictRejected, VerdictResolved, VerdictAnswered, VerdictAsked} {
		if counts[kind] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[kind], kind))
		}
	}
	return fmt.Sprintf("Verdicts on %s: %s.", batch.Path, strings.Join(parts, ", "))
}

// RoundReports counts the assignee's decisions and questions since the human last wrote in the thread, by file, so submit can refuse a round with nothing in it.
func (s *Store) RoundReports(threadID string) (decisions, questions int, files []string, err error) {
	rows, err := s.db.Query(`
		SELECT kind, COALESCE(data, '') FROM lg_messages
		WHERE thread_id = ? AND id > COALESCE((SELECT MAX(id) FROM lg_messages WHERE thread_id = ? AND tab_id = ?), 0)
			AND kind IN (?, ?)
		ORDER BY id`, threadID, threadID, HumanAuthor, MessageDecision, MessageQuestion)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, data string
		if err := rows.Scan(&kind, &data); err != nil {
			return 0, 0, nil, err
		}
		if kind == MessageDecision {
			decisions++
		} else {
			questions++
		}
		var report BlockReport
		if json.Unmarshal([]byte(data), &report) == nil && report.Path != "" && !containsString(files, report.Path) {
			files = append(files, report.Path)
		}
	}
	return decisions, questions, files, rows.Err()
}

// BlockStanding is where one block stands by the latest word on it in the thread.
type BlockStanding struct {
	Path    string
	Block   int
	Kind    string // decision, question, or a verdict
	Message int64
}

// BlockStandings returns, per file and block, the latest decision, question or verdict on it, ordered by file then block.
func (s *Store) BlockStandings(threadID string) ([]BlockStanding, error) {
	rows, err := s.db.Query(`SELECT id, kind, data FROM lg_messages WHERE thread_id = ? AND kind IN (?, ?, ?) AND data IS NOT NULL ORDER BY id`,
		threadID, MessageDecision, MessageQuestion, MessageVerdicts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type key struct {
		path  string
		block int
	}
	latest := map[key]BlockStanding{}
	for rows.Next() {
		var id int64
		var kind, data string
		if err := rows.Scan(&id, &kind, &data); err != nil {
			return nil, err
		}
		if kind == MessageVerdicts {
			var batch Verdicts
			if json.Unmarshal([]byte(data), &batch) != nil {
				continue
			}
			for _, v := range batch.Verdicts {
				latest[key{batch.Path, v.Block}] = BlockStanding{Path: batch.Path, Block: v.Block, Kind: v.Verdict, Message: id}
			}
			continue
		}
		var report BlockReport
		if json.Unmarshal([]byte(data), &report) != nil {
			continue
		}
		latest[key{report.Path, report.Block}] = BlockStanding{Path: report.Path, Block: report.Block, Kind: kind, Message: id}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]BlockStanding, 0, len(latest))
	for _, b := range latest {
		out = append(out, b)
	}
	sortStandings(out)
	return out, nil
}

func sortStandings(s []BlockStanding) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && (s[j].Path < s[j-1].Path || (s[j].Path == s[j-1].Path && s[j].Block < s[j-1].Block)); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// FormatStandings is the part of `lgrass git conflict show` that says what the human already settled and what waits for the model, so a model resuming the conflict needs no thread history to know it.
func FormatStandings(standings []BlockStanding) string {
	if len(standings) == 0 {
		return ""
	}
	type group struct{ keep, waiting []string }
	var order []string
	groups := map[string]*group{}
	for _, b := range standings {
		g := groups[b.Path]
		if g == nil {
			g = &group{}
			groups[b.Path] = g
			order = append(order, b.Path)
		}
		switch b.Kind {
		case VerdictAccepted:
			g.keep = append(g.keep, fmt.Sprintf("block %d (you wrote it, accepted)", b.Block))
		case VerdictResolved:
			g.keep = append(g.keep, fmt.Sprintf("block %d (resolved by the human)", b.Block))
		case VerdictRejected, VerdictAnswered, VerdictAsked:
			g.waiting = append(g.waiting, fmt.Sprintf("block %d (%s, see #%d)", b.Block, b.Kind, b.Message))
		}
	}
	var lines []string
	for _, path := range order {
		g := groups[path]
		if len(g.keep) == 0 && len(g.waiting) == 0 {
			continue
		}
		lines = append(lines, path)
		if len(g.keep) > 0 {
			lines = append(lines, "  settled, keep exactly as is: "+strings.Join(g.keep, ", "))
		}
		if len(g.waiting) > 0 {
			lines = append(lines, "  waiting for you: "+strings.Join(g.waiting, ", "))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "The human's verdicts so far:\n" + strings.Join(lines, "\n")
}

// How `lgrass thread read` prints a decision, question or verdicts message for the model: the header suffix, and the lines that replace the stored body.
func formatBlockMessage(m Message) (suffix string, body string) {
	switch m.Kind {
	case MessageDecision, MessageQuestion:
		var r BlockReport
		if json.Unmarshal([]byte(m.Data), &r) != nil {
			return " " + m.Kind, m.Body
		}
		suffix = fmt.Sprintf(" %s on %s block %d", m.Kind, r.Path, r.Block)
		body = m.Body
		for i, o := range r.Options {
			body += fmt.Sprintf("\n  option %d: %s", i+1, o)
		}
		return suffix, body
	case MessageVerdicts:
		var batch Verdicts
		if json.Unmarshal([]byte(m.Data), &batch) != nil {
			return " verdicts", m.Body
		}
		lines := make([]string, len(batch.Verdicts))
		for i, v := range batch.Verdicts {
			lines[i] = formatVerdict(v)
		}
		return " verdicts on " + batch.Path, strings.Join(lines, "\n")
	}
	return "", m.Body
}

func formatVerdict(v Verdict) string {
	switch v.Verdict {
	case VerdictAccepted:
		return fmt.Sprintf("block %d: accepted, keep it exactly as is", v.Block)
	case VerdictResolved:
		return fmt.Sprintf("block %d: resolved by the human, keep it exactly as is", v.Block)
	case VerdictRejected:
		return fmt.Sprintf("block %d: rejected, revise it and decide again: %s", v.Block, v.Text)
	case VerdictAnswered:
		return fmt.Sprintf("block %d: the human answered your question #%d: %s", v.Block, v.Question, v.Text)
	case VerdictAsked:
		return fmt.Sprintf("block %d: the human asks: %s", v.Block, v.Text)
	}
	return fmt.Sprintf("block %d: %s", v.Block, v.Verdict)
}

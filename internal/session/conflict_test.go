// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/testenv"
)

const conflictScope = "conflict:merge:main:feature"

func conflictStore(t *testing.T) *Store {
	t.Helper()
	store := openTestStore(t)
	for _, tab := range []string{tabA, tabB} {
		if err := store.RegisterTab(tab, "claude"); err != nil {
			t.Fatalf("RegisterTab: %v", err)
		}
	}
	return store
}

func ownedRows(t *testing.T, store *Store, tab string) Pending {
	t.Helper()
	p, err := store.PendingForTab(tab)
	if err != nil {
		t.Fatalf("PendingForTab: %v", err)
	}
	return p
}

// A store at version 3 keeps its threads, and a workgroup thread is marked as one.
func TestMigrationV4KindsExistingThreads(t *testing.T) {
	testenv.Home(t)
	saved := migrations
	migrations = migrations[:3]
	store, err := Open(DBPath(), testProjectID)
	migrations = saved
	if err != nil {
		t.Fatalf("Open at v3: %v", err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO lg_threads (id, project_id, title, group_id, created_by, created_at) VALUES ('grp001', 'testproj', 'Group', 7, 'x', '2026-01-01T00:00:00Z');
		INSERT INTO lg_threads (id, project_id, title, created_by, created_at) VALUES ('adh001', 'testproj', 'Adhoc', 'x', '2026-01-01T00:00:00Z');
		INSERT INTO lg_messages (thread_id, tab_id, body, created_at) VALUES ('adh001', 'x', 'old body', '2026-01-01T00:00:00Z');`); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	store.Close()

	store, err = Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	if v := storeVersion(t, store); v != len(migrations) {
		t.Errorf("version = %d, want %d", v, len(migrations))
	}
	if g, _ := store.ThreadByID("grp001"); g.Kind != ThreadWorkgroup {
		t.Errorf("group thread kind = %q, want workgroup", g.Kind)
	}
	a, _ := store.ThreadByID("adh001")
	if a.Kind != ThreadAdHoc || a.Closed() || a.Scope != "" || a.Assignee != "" {
		t.Errorf("ad hoc thread = %+v, want an open adhoc thread with no scope", a)
	}
	msgs, _, err := store.ReadThread("adh001", 0, 10)
	if err != nil || len(msgs) != 1 || msgs[0].Kind != MessagePlain || msgs[0].Anchor != nil || msgs[0].Files != nil {
		t.Errorf("old message = %+v, %v, want a plain message with no anchor or files", msgs, err)
	}
}

func TestCreateGroupMarksItsThreadWorkgroup(t *testing.T) {
	store := openTestStore(t)
	g, err := store.CreateGroup("Kinds", Member{TabID: tabA, Label: "lead", Vendor: "claude"}, []Member{{TabID: tabB, Label: "one", Vendor: "claude"}})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if th, _ := store.ThreadByID(g.ThreadID); th.Kind != ThreadWorkgroup {
		t.Errorf("kind = %q, want workgroup", th.Kind)
	}
}

func TestOpenConflictReusesTheScopeAndClosesAnEndedOne(t *testing.T) {
	store := conflictStore(t)

	first, err := store.OpenConflict(conflictScope, tabA, `{"a.go":"1"}`)
	if err != nil {
		t.Fatalf("OpenConflict: %v", err)
	}
	if first.Kind != ThreadConflict || first.Scope != conflictScope || first.Assignee != tabA || first.CreatedBy != HumanAuthor || first.Title != "merge:main:feature" {
		t.Errorf("thread = %+v, want an open conflict thread handed to tabA", first)
	}
	again, err := store.OpenConflict(conflictScope, tabB, `{"a.go":"2"}`)
	if err != nil || again.ID != first.ID || again.Assignee != tabB {
		t.Errorf("reopen = %+v, %v, want the same thread handed to tabB", again, err)
	}
	if b, _ := store.ConflictBaseline(first.ID); b != `{"a.go":"2"}` {
		t.Errorf("baseline = %q, want the new hand-off's", b)
	}

	next, err := store.OpenConflict("conflict:rebase:feature:main", tabA, "")
	if err != nil || next.ID == first.ID {
		t.Fatalf("new scope = %+v, %v, want a new thread", next, err)
	}
	if old, _ := store.ThreadByID(first.ID); !old.Closed() {
		t.Error("the thread of the ended scope stayed open")
	}
	open, err := store.OpenConflictThread()
	if err != nil || open.ID != next.ID {
		t.Errorf("open thread = %+v, %v, want only the new one", open, err)
	}
	// sqlite itself holds one open conflict thread per project.
	if _, err := store.db.Exec(`INSERT INTO lg_threads (id, project_id, title, created_by, created_at, kind, scope) VALUES ('dup001', ?, 't', 'human', 'x', 'conflict', 'conflict:x')`, testProjectID); err == nil {
		t.Error("a second open conflict thread was stored")
	}
}

func TestOpenConflictRefusesBadScopeAndUnknownTab(t *testing.T) {
	store := conflictStore(t)
	if _, err := store.OpenConflict("merge:main:feature", tabA, ""); err == nil {
		t.Error("a scope without the conflict prefix was accepted")
	}
	if _, err := store.OpenConflict(conflictScope, tabC, ""); err == nil {
		t.Error("an unregistered tab was handed the conflict")
	}
}

func TestConflictThreadForRefusesOtherTabs(t *testing.T) {
	store := conflictStore(t)
	if _, err := store.ConflictThreadFor(tabA); !errors.Is(err, ErrNoConflict) {
		t.Errorf("no conflict: err = %v, want ErrNoConflict", err)
	}
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	if got, err := store.ConflictThreadFor(tabA); err != nil || got.ID != th.ID {
		t.Errorf("assignee: %+v, %v", got, err)
	}
	var notAssignee NotAssigneeError
	if _, err := store.ConflictThreadFor(tabB); !errors.As(err, &notAssignee) || !strings.Contains(err.Error(), "aaaaaaaa") {
		t.Errorf("other tab: err = %v, want a refusal naming the assignee", err)
	}
	if _, err := store.ConflictThreadFor(""); err == nil {
		t.Error("a call without a tab was accepted")
	}
	store.CloseConflict(conflictScope, th.ID)
	if _, err := store.ConflictThreadFor(tabA); !errors.Is(err, ErrNoConflict) {
		t.Errorf("closed: err = %v, want ErrNoConflict", err)
	}
}

func TestHumanMessagesMentionAndWakeTheAssignee(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	anchor := &Anchor{Path: "a.go", StartLine: 3, EndLine: 4, Quote: "x := 1\ny := 2"}
	msgs, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "keep theirs here", Anchor: anchor}, {Body: "overall fine"}})
	if err != nil || len(msgs) != 2 {
		t.Fatalf("PostHumanMessages: %v, %v", msgs, err)
	}
	read, _, _ := store.ReadThread(th.ID, 0, 10)
	if read[1].TabID != HumanAuthor || read[1].Kind != MessagePlain || read[1].Anchor == nil || *read[1].Anchor != *anchor || read[0].Anchor != nil {
		t.Errorf("stored = %+v, want the human's anchored message then a plain one", read)
	}
	if len(read[0].Mentions) != 1 || read[0].Mentions[0] != tabA {
		t.Errorf("mentions = %v, want the assignee", read[0].Mentions)
	}
	p := ownedRows(t, store, tabA)
	if len(p.Threads) != 1 || p.Threads[0].You.Count != 2 {
		t.Errorf("assignee owed %+v, want 2 for you", p.Threads)
	}
	if !ownedRows(t, store, tabB).Empty() {
		t.Error("a tab other than the assignee was notified")
	}
	if line := FormatNotification(p.Threads); !strings.Contains(line, "2 for you from human") {
		t.Errorf("nudge line = %q", line)
	}

	if _, err := store.PostHumanMessages("conflict:other", []HumanMessage{{Body: "x"}}); err == nil {
		t.Error("a post for another scope was accepted")
	}
	if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "x", Anchor: &Anchor{Path: "a.go", StartLine: 5, EndLine: 4}}}); err == nil {
		t.Error("an anchor ending before it starts was accepted")
	}
	if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "ok"}, {Body: " "}}); err == nil {
		t.Error("a blank message was accepted")
	}
	if after, _, _ := store.ReadThread(th.ID, 0, 10); len(after) != 2 {
		t.Errorf("a refused batch stored %d messages, want none added", len(after)-2)
	}
}

// A reply answers one message of the same thread and is stored and read with that id. One aimed anywhere else refuses the whole batch.
func TestHumanReplyAnswersAMessageOfTheSameThread(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	anchor := Anchor{Path: "a.go", StartLine: 3, EndLine: 7, Quote: "<<<<<<< ours"}
	note, err := store.PostQuestion(tabA, "a.go", 1, "10 or 3? Which threshold?", nil)
	if err != nil {
		t.Fatalf("PostQuestion: %v", err)
	}
	if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "use 10", Anchor: &anchor, ReplyTo: note.ID}}); err != nil {
		t.Fatalf("reply: %v", err)
	}
	msgs, _, _ := store.ReadThread(th.ID, 0, 10)
	if msgs[0].ReplyTo != note.ID || msgs[0].TabID != HumanAuthor || msgs[1].ReplyTo != 0 {
		t.Errorf("messages = %+v, want the reply pointing at note #%d and the note pointing nowhere", msgs, note.ID)
	}
	text := FormatThreadRead(th, msgs, false, false)
	if !strings.Contains(text, fmt.Sprintf("(replying to #%d)", note.ID)) {
		t.Errorf("thread read = %q, want the reply marked", text)
	}

	otherID, err := store.CreateThread(tabA, "Elsewhere", "hello")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	elsewhere, _, _ := store.ReadThread(otherID, 0, 1)
	for _, bad := range []int64{elsewhere[0].ID, 99999} {
		if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "ok"}, {Body: "answer", ReplyTo: bad}}); err == nil {
			t.Errorf("a reply to message #%d outside the thread was accepted", bad)
		}
	}
	if after, _, _ := store.ReadThread(th.ID, 0, 10); len(after) != 2 {
		t.Errorf("a refused batch stored %d messages, want none added", len(after)-2)
	}
}

func TestReportsAndSubmitsWakeNobodyAndOnlyTheAssigneePosts(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "hi"}})

	if _, err := store.PostDecision(tabA, "a.go", 2, "kept ours, theirs renamed it"); err != nil {
		t.Fatalf("PostDecision: %v", err)
	}
	if _, err := store.PostSubmit(tabA, "Round done.", []string{"a.go"}); err != nil {
		t.Fatalf("PostSubmit: %v", err)
	}
	if _, err := store.PostMessage(tabA, th.ID, "which side do you want?"); err != nil {
		t.Fatalf("PostMessage: %v", err)
	}
	if !ownedRows(t, store, tabB).Empty() {
		t.Error("a model post in a conflict thread notified a tab")
	}
	msgs, _, _ := store.ReadThread(th.ID, 0, 10)
	if msgs[0].Kind != MessagePlain || msgs[1].Kind != MessageSubmit || len(msgs[1].Files) != 1 || msgs[1].Files[0] != "a.go" || msgs[2].Kind != MessageDecision || msgs[2].Data != `{"path":"a.go","block":2}` {
		t.Errorf("messages = %+v", msgs)
	}

	if _, err := store.PostDecision(tabB, "a.go", 1, "x"); err == nil {
		t.Error("a decision from another tab was accepted")
	}
	if _, err := store.PostQuestion(tabA, "a.go", 0, "x", nil); err == nil {
		t.Error("a question on block 0 was accepted")
	}
	if _, err := store.PostQuestion(tabA, "a.go", 1, "x", []string{"a", " "}); err == nil {
		t.Error("a question with an empty option was accepted")
	}
	if _, err := store.PostSubmit(tabA, "x", nil); err == nil {
		t.Error("a submission with no files was accepted")
	}
	if _, err := store.PostMessage(tabB, th.ID, "me too"); err == nil {
		t.Error("another tab posted into the conflict thread")
	}
	if _, err := store.PostMessage(tabA, th.ID, "ask !>>bbbbbbbb<<!"); err == nil {
		t.Error("a mention in a conflict thread was accepted")
	}
	store.CloseConflict(conflictScope, th.ID)
	if _, err := store.PostMessage(tabA, th.ID, "late"); err == nil {
		t.Error("a post into a closed conflict thread was accepted")
	}
}

func TestCloseConflictMatchesScopeAndThread(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	if closed, err := store.CloseConflict("conflict:other", ""); err != nil || closed {
		t.Errorf("other scope: %v, %v, want nothing closed", closed, err)
	}
	if closed, _ := store.CloseConflict(conflictScope, "nosuch"); closed {
		t.Error("closed with another thread id")
	}
	if closed, err := store.CloseConflict(conflictScope, th.ID); err != nil || !closed {
		t.Errorf("close: %v, %v", closed, err)
	}
}

func TestThreadReadShowsAnchorsAndKinds(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "why this?", Anchor: &Anchor{Path: "a.go", StartLine: 4, EndLine: 5, Quote: "one\ntwo"}}})
	store.PostDecision(tabA, "a.go", 2, "kept both")
	store.PostQuestion(tabA, "b.go", 1, "which threshold?", []string{"10", "3"})
	store.PostSubmit(tabA, "Submitted.", []string{"a.go", "b.go"})
	th, _ = store.ThreadByID(th.ID)
	msgs, more, _ := store.ReadThread(th.ID, 0, 10)
	out := FormatThreadRead(th, msgs, more, false)
	for _, want := range []string{"opened by human", "human on a.go lines 4-5 (mentions aaaaaaaa):\n> one\n> two\nwhy this?", "aaaaaaaa decision on a.go block 2:\nkept both", "aaaaaaaa question on b.go block 1:\nwhich threshold?\n  option 1: 10\n  option 2: 3", "aaaaaaaa submitted a.go, b.go:\nSubmitted."} {
		if !strings.Contains(out, want) {
			t.Errorf("read missing %q in:\n%s", want, out)
		}
	}
}

func TestVerdictsAreCheckedAndStandingsFollowTheLatestWord(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	q, _ := store.PostQuestion(tabA, "a.go", 1, "which?", []string{"x", "y"})
	store.PostDecision(tabA, "a.go", 2, "took both")
	store.PostDecision(tabA, "b.go", 1, "took ours")

	bad := []Verdicts{
		{Path: "", Verdicts: []Verdict{{Block: 1, Verdict: VerdictAccepted, Digest: "d"}}},
		{Path: "a.go"},
		{Path: "a.go", Verdicts: []Verdict{{Block: 0, Verdict: VerdictAccepted, Digest: "d"}}},
		{Path: "a.go", Verdicts: []Verdict{{Block: 2, Verdict: VerdictAccepted}}},
		{Path: "a.go", Verdicts: []Verdict{{Block: 2, Verdict: VerdictRejected, Text: " "}}},
		{Path: "a.go", Verdicts: []Verdict{{Block: 1, Verdict: VerdictAnswered, Text: "x"}}},
		{Path: "a.go", Verdicts: []Verdict{{Block: 1, Verdict: VerdictAnswered, Text: "x", Question: q.ID + 1}}},
		{Path: "a.go", Verdicts: []Verdict{{Block: 1, Verdict: "maybe"}}},
	}
	for i, batch := range bad {
		if _, err := store.PostVerdicts(conflictScope, batch, false); err == nil {
			t.Errorf("bad batch %d was accepted", i)
		}
	}
	if _, err := store.PostVerdicts("conflict:other", Verdicts{Path: "a.go", Verdicts: []Verdict{{Block: 2, Verdict: VerdictAccepted, Digest: "d"}}}, false); err == nil {
		t.Error("verdicts for another scope were accepted")
	}

	msg, err := store.PostVerdicts(conflictScope, Verdicts{Path: "a.go", Verdicts: []Verdict{
		{Block: 1, Verdict: VerdictAnswered, Text: "y", Question: q.ID},
		{Block: 2, Verdict: VerdictAccepted, Digest: "d2"},
	}}, false)
	if err != nil {
		t.Fatalf("PostVerdicts: %v", err)
	}
	if msg.Body != "Verdicts on a.go: 1 accepted, 1 answered." || msg.Kind != MessageVerdicts {
		t.Errorf("verdicts message = %+v", msg)
	}
	if rows := ownedRows(t, store, tabA); rows.Empty() {
		t.Error("verdicts did not wake the assignee")
	}
	store.PostVerdicts(conflictScope, Verdicts{Path: "b.go", Verdicts: []Verdict{{Block: 1, Verdict: VerdictResolved, Digest: "d3"}}}, true)

	standings, err := store.BlockStandings(th.ID)
	if err != nil {
		t.Fatalf("BlockStandings: %v", err)
	}
	want := []BlockStanding{{"a.go", 1, VerdictAnswered, msg.ID}, {"a.go", 2, VerdictAccepted, msg.ID}, {"b.go", 1, VerdictResolved, msg.ID + 1}}
	if fmt.Sprint(standings) != fmt.Sprint(want) {
		t.Errorf("standings = %v, want %v", standings, want)
	}
	text := FormatStandings(standings)
	for _, line := range []string{"a.go\n  settled, keep exactly as is: block 2 (you wrote it, accepted)\n  waiting for you: block 1 (answered, see #", "b.go\n  settled, keep exactly as is: block 1 (resolved by the human)"} {
		if !strings.Contains(text, line) {
			t.Errorf("standings text missing %q in:\n%s", line, text)
		}
	}

	decisions, questions, files, _ := store.RoundReports(th.ID)
	if decisions+questions != 0 || len(files) != 0 {
		t.Errorf("round after the human wrote = %d, %d, %v, want nothing", decisions, questions, files)
	}
	store.PostDecision(tabA, "a.go", 1, "took y")
	if decisions, _, files, _ = store.RoundReports(th.ID); decisions != 1 || len(files) != 1 || files[0] != "a.go" {
		t.Errorf("round = %d, %v", decisions, files)
	}
}

func TestAnchorCarriesPaneAndBlockAndAllowsNoLines(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	msgs, err := store.PostHumanMessages(conflictScope, []HumanMessage{
		{Body: "why this line?", Anchor: &Anchor{Path: "a.go", StartLine: 3, EndLine: 4, Quote: "x\ny", Pane: "right", Block: 2}},
		{Body: "about the block", Anchor: &Anchor{Path: "a.go", Block: 2}},
		{Body: "about the file", Anchor: &Anchor{Path: "a.go"}},
	})
	if err != nil || len(msgs) != 3 {
		t.Fatalf("PostHumanMessages: %v, %v", msgs, err)
	}
	read, _, _ := store.ReadThread(th.ID, 0, 10)
	want := []Anchor{{Path: "a.go"}, {Path: "a.go", Block: 2}, {Path: "a.go", StartLine: 3, EndLine: 4, Quote: "x\ny", Pane: "right", Block: 2}}
	for i, m := range read {
		if m.Anchor == nil || *m.Anchor != want[i] {
			t.Errorf("message %d anchor = %+v, want %+v", i, m.Anchor, want[i])
		}
	}
	text := FormatThreadRead(th, read, false, false)
	for _, line := range []string{" on a.go block 2 (right pane) lines 3-4", " on a.go block 2 (mentions", " on a.go (mentions"} {
		if !strings.Contains(text, line) {
			t.Errorf("thread read = %q, want %q", text, line)
		}
	}
	for _, bad := range []Anchor{
		{Path: "a.go", Pane: "bottom"},
		{Path: "a.go", Block: -1},
		{Path: "a.go", StartLine: 0, EndLine: 3},
		{Path: " "},
	} {
		if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "x", Anchor: &bad}}); err == nil {
			t.Errorf("anchor %+v was accepted", bad)
		}
	}
}

func TestReplyKeepsTheAnchorAndAnswersOnlyAHumanComment(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	anchor := Anchor{Path: "a.go", StartLine: 3, EndLine: 4, Quote: "x\ny", Pane: "left", Block: 2}
	comment, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "why?", Anchor: &anchor}})
	if err != nil {
		t.Fatalf("PostHumanMessages: %v", err)
	}
	reply, err := store.PostConflictReply(tabA, comment[0].ID, "because the other side renamed it")
	if err != nil {
		t.Fatalf("PostConflictReply: %v", err)
	}
	if reply.ReplyTo != comment[0].ID || reply.Anchor == nil || *reply.Anchor != anchor || len(reply.Mentions) != 0 {
		t.Errorf("reply = %+v, want it answering #%d with the same anchor and no mention", reply, comment[0].ID)
	}
	if !ownedRows(t, store, tabB).Empty() {
		t.Error("a reply woke another tab")
	}

	var notAssignee NotAssigneeError
	if _, err := store.PostConflictReply(tabB, comment[0].ID, "me too"); !errors.As(err, &notAssignee) {
		t.Errorf("a reply from another tab = %v, want NotAssigneeError", err)
	}
	decision, _ := store.PostDecision(tabA, "a.go", 1, "took theirs")
	for name, id := range map[string]int64{"its own message": decision.ID, "an unknown message": 99999} {
		if _, err := store.PostConflictReply(tabA, id, "x"); err == nil {
			t.Errorf("a reply to %s was accepted", name)
		}
	}
	if _, err := store.PostConflictReply(tabA, comment[0].ID, " "); err == nil {
		t.Error("an empty reply was accepted")
	}
	msgs, _, _ := store.ReadThread(th.ID, 0, 10)
	if text := FormatThreadRead(th, msgs, false, false); !strings.Contains(text, "(replying to #") {
		t.Errorf("thread read = %q, want the reply marked", text)
	}
}

func TestQuietHumanMessagesWakeNobody(t *testing.T) {
	store := conflictStore(t)
	th, _ := store.OpenConflict(conflictScope, tabA, "")
	if _, err := store.PostHumanMessagesQuiet(conflictScope, []HumanMessage{{Body: "a comment", Anchor: &Anchor{Path: "a.go", Block: 1}}}); err != nil {
		t.Fatalf("PostHumanMessagesQuiet: %v", err)
	}
	if !ownedRows(t, store, tabA).Empty() {
		t.Error("a quiet post woke the assignee")
	}
	msgs, _, _ := store.ReadThread(th.ID, 0, 10)
	if len(msgs) != 1 || len(msgs[0].Mentions) != 0 {
		t.Errorf("messages = %+v, want the comment stored with no mention", msgs)
	}
	if _, err := store.PostHumanMessages(conflictScope, []HumanMessage{{Body: "now wake"}}); err != nil {
		t.Fatalf("PostHumanMessages: %v", err)
	}
	if ownedRows(t, store, tabA).Empty() {
		t.Error("a normal post did not wake the assignee")
	}
}

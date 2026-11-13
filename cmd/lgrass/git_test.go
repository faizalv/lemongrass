// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"encoding/json"
	"github.com/faizalv/lemongrass/internal/ipc"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

const (
	gitTabA  = "aaaaaaaa-1111-4111-8111-111111111111"
	gitTabB  = "bbbbbbbb-2222-4222-8222-222222222222"
	gitScope = "conflict:merge:main:feature"
)

// conflictRepo is a repo stopped in a merge of feature into main with a.txt in conflict and b.txt merged cleanly.
func conflictRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null")
		cmd.Run()
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("a.txt", "one\nrate\nthree\n")
	write("b.txt", "b\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "feature")
	write("a.txt", "one\nTAX\nthree\n")
	write("b.txt", "b feature\n")
	run("commit", "-q", "-am", "rename")
	run("checkout", "-q", "main")
	write("a.txt", "one\nTAX_RATE\nthree\n")
	run("commit", "-q", "-am", "main side")
	run("merge", "-q", "feature")
	out, _ := exec.Command("git", "-C", dir, "diff", "--name-only", "--diff-filter=U").Output()
	if strings.TrimSpace(string(out)) != "a.txt" {
		t.Fatalf("setup: unmerged = %q, want a.txt", out)
	}
	return dir
}

type appReply func(req map[string]any) map[string]any

// fakeConflictApp stands in for the app on app.sock, records each request and answers with reply.
func fakeConflictApp(t *testing.T, reply appReply) (string, *[]map[string]any) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "app.sock")
	l, err := ipc.Listen(sock)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	var got []map[string]any
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			var req map[string]any
			json.NewDecoder(conn).Decode(&req)
			got = append(got, req)
			json.NewEncoder(conn).Encode(reply(req))
			conn.Close()
		}
	}()
	return sock, &got
}

func gitEnv(t *testing.T, sock, tab string) (conflictEnv, *bytes.Buffer, *int) {
	t.Helper()
	store := openTestStore(t)
	for _, id := range []string{gitTabA, gitTabB} {
		store.RegisterTab(id, "claude")
	}
	dir := conflictRepo(t)
	var out bytes.Buffer
	notified := 0
	return conflictEnv{store: store, socket: sock, projectPath: "/proj", dir: dir, tab: tab, out: &out, notify: func() { notified++ }}, &out, &notified
}

func claimReply(grant map[string]any) appReply {
	return func(req map[string]any) map[string]any {
		if req["op"] != "conflict_claim" {
			return map[string]any{"ok": false, "error": "unexpected op"}
		}
		return map[string]any{"ok": true, "conflict": grant}
	}
}

func startConflict(t *testing.T, e conflictEnv) string {
	t.Helper()
	sock, _ := fakeConflictApp(t, claimReply(map[string]any{"action": "start", "project_path": "/proj", "scope": gitScope, "tab_id": gitTabA}))
	e.socket = sock
	var out bytes.Buffer
	e.out = &out
	if err := conflictApp(e, []string{"--token", "t1", "--json"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got["threadId"] == "" {
		t.Fatalf("start output %q, %v", out.String(), err)
	}
	return got["threadId"]
}

func TestAppStartPostCloseThroughClaimedTokens(t *testing.T) {
	e, _, notified := gitEnv(t, "", gitTabA)
	id := startConflict(t, e)

	var status bytes.Buffer
	e.out = &status
	conflictStatus(e, true)
	if !strings.Contains(status.String(), `"threadId":"`+id+`"`) || !strings.Contains(status.String(), `"assignee":"`+gitTabA+`"`) {
		t.Errorf("status = %s", status.String())
	}

	sock, got := fakeConflictApp(t, claimReply(map[string]any{"action": "post", "project_path": "/proj", "scope": gitScope,
		"messages": []map[string]any{{"body": "why TAX?", "anchor": map[string]any{"path": "a.txt", "startLine": 2, "endLine": 2, "quote": "TAX"}}, {"body": "overall", "anchor": nil}}}))
	e.socket = sock
	var out bytes.Buffer
	e.out = &out
	if err := conflictApp(e, []string{"--token", "t2", "--json"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if strings.TrimSpace(out.String()) != `{"ok":true}` || *notified != 1 {
		t.Errorf("post output %q, notified %d", out.String(), *notified)
	}
	if req := (*got)[0]; req["op"] != "conflict_claim" || req["token"] != "t2" {
		t.Errorf("claim request = %v", req)
	}
	msgs, _, _ := e.store.ReadThread(id, 0, 10)
	if len(msgs) != 2 || msgs[1].TabID != session.HumanAuthor || msgs[1].Anchor == nil || msgs[1].Anchor.Quote != "TAX" {
		t.Errorf("thread = %+v", msgs)
	}

	sock, _ = fakeConflictApp(t, claimReply(map[string]any{"action": "close", "project_path": "/proj", "scope": gitScope, "thread_id": id}))
	e.socket = sock
	if err := conflictApp(e, []string{"--token", "t3", "--json"}); err != nil {
		t.Fatalf("close: %v", err)
	}
	status.Reset()
	e.out = &status
	conflictStatus(e, true)
	if strings.TrimSpace(status.String()) != "null" {
		t.Errorf("status after close = %q, want null", status.String())
	}
}

func TestAppRefusesARefusedTokenAndAnotherProject(t *testing.T) {
	e, _, _ := gitEnv(t, "", gitTabA)
	sock, _ := fakeConflictApp(t, func(map[string]any) map[string]any {
		return map[string]any{"ok": false, "error": "unknown or expired token"}
	})
	e.socket = sock
	if err := conflictApp(e, []string{"--token", "bad", "--json"}); err == nil || !strings.Contains(err.Error(), "expired token") {
		t.Errorf("refused token: err = %v", err)
	}
	sock, _ = fakeConflictApp(t, claimReply(map[string]any{"action": "start", "project_path": "/elsewhere", "scope": gitScope, "tab_id": gitTabA}))
	e.socket = sock
	if err := conflictApp(e, []string{"--token", "t", "--json"}); err == nil || !strings.Contains(err.Error(), "another project") {
		t.Errorf("other project: err = %v", err)
	}
	if err := conflictApp(e, nil); err == nil {
		t.Error("app ran without a token")
	}
	if _, err := e.store.OpenConflictThread(); err == nil {
		t.Error("a refused start opened a thread")
	}
}

func TestModelCommandsRefuseAnUnpickedTabAndNoConflict(t *testing.T) {
	sock, got := fakeConflictApp(t, func(map[string]any) map[string]any { return map[string]any{"ok": true, "text": "x"} })
	e, _, _ := gitEnv(t, sock, gitTabB)
	if err := conflictShow(e); err == nil || !strings.Contains(err.Error(), "no conflict") {
		t.Errorf("no conflict: err = %v", err)
	}
	startConflict(t, e)
	e.socket = sock
	for name, run := range map[string]func() error{
		"show":   func() error { return conflictShow(e) },
		"decide": func() error { return conflictDecide(e, []string{"a.txt", "1", "why"}) },
		"ask":    func() error { return conflictAsk(e, []string{"a.txt", "1", "which?"}) },
		"submit": func() error { return conflictSubmit(e) },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "handed to aaaaaaaa") {
			t.Errorf("%s from another tab: err = %v", name, err)
		}
	}
	e.tab = ""
	if err := conflictShow(e); err == nil || !strings.Contains(err.Error(), "needs a lemongrass tab") {
		t.Errorf("no tab: err = %v", err)
	}
	if len(*got) != 0 {
		t.Errorf("a refused command reached the app: %v", *got)
	}
}

// blockApp answers conflict_block with what the test says block 1 of a.txt holds, and every other op with ok.
func blockApp(t *testing.T, state *string) (string, *[]map[string]any) {
	return fakeConflictApp(t, func(req map[string]any) map[string]any {
		switch req["op"] {
		case "conflict_show":
			return map[string]any{"ok": true, "text": "the merge of feature into main\n"}
		case "conflict_block":
			if req["block"] != float64(1) {
				return map[string]any{"ok": false, "error": "a.txt has 1 block"}
			}
			return map[string]any{"ok": true, "blocks": 1, "state": *state}
		}
		return map[string]any{"ok": true}
	})
}

func TestDecideAskAndSubmitForTheAssignee(t *testing.T) {
	state := "markers"
	sock, got := blockApp(t, &state)
	e, out, _ := gitEnv(t, sock, gitTabA)
	id := startConflict(t, e)
	e.socket = sock

	if err := conflictShow(e); err != nil || !strings.Contains(out.String(), "the merge of feature into main") {
		t.Fatalf("show: %v, %q", err, out.String())
	}
	if req := (*got)[0]; req["op"] != "conflict_show" || req["tab_id"] != gitTabA || req["scope"] != gitScope || req["thread_id"] != id || req["project_path"] != "/proj" {
		t.Errorf("show request = %v", req)
	}
	if err := conflictSubmit(e); err == nil || !strings.Contains(err.Error(), "nothing to submit") {
		t.Errorf("submit before any report: err = %v", err)
	}

	if err := conflictDecide(e, []string{"a.txt", "1", "took TAX"}); err == nil || !strings.Contains(err.Error(), "still holds its conflict markers") {
		t.Errorf("decide on a block with markers: err = %v", err)
	}
	if err := conflictAsk(e, []string{"a.txt", "1", "which name?", "--option", "TAX", "--option", "TAX_RATE"}); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if req := (*got)[len(*got)-1]; req["op"] != "conflict_block" || req["path"] != "a.txt" || req["block"] != float64(1) || req["scope"] != gitScope {
		t.Errorf("block request = %v", req)
	}
	state = "written"
	if err := conflictAsk(e, []string{"a.txt", "1", "which?"}); err == nil || !strings.Contains(err.Error(), "no longer holds") {
		t.Errorf("ask on a written block: err = %v", err)
	}
	if err := conflictDecide(e, []string{"a.txt", "1", "theirs renamed the rate to TAX"}); err != nil {
		t.Fatalf("decide: %v", err)
	}
	if err := conflictDecide(e, []string{"a.txt", "2", "x"}); err == nil || !strings.Contains(err.Error(), "has 1 block") {
		t.Errorf("decide past the last block: err = %v", err)
	}
	if err := conflictDecide(e, []string{"a.txt", "x", "x"}); err == nil || !strings.Contains(err.Error(), "not a block number") {
		t.Errorf("decide with no number: err = %v", err)
	}
	if err := conflictDecide(e, []string{"b.txt", "1", "x"}); err == nil || !strings.Contains(err.Error(), "not a conflicted file") {
		t.Errorf("decide on a clean file: err = %v", err)
	}
	if err := conflictDecide(e, []string{"../a.txt", "1", "x"}); err == nil {
		t.Error("decide outside the repo was accepted")
	}

	if err := conflictSubmit(e); err != nil {
		t.Fatalf("submit: %v", err)
	}
	last := (*got)[len(*got)-1]
	if last["op"] != "conflict_submit" || last["scope"] != gitScope || last["tab_id"] != gitTabA {
		t.Errorf("submit request = %v", last)
	}
	if files, _ := last["files"].([]any); len(files) != 1 || files[0] != "a.txt" {
		t.Errorf("submit files = %v, want a.txt", last["files"])
	}

	msgs, _, _ := e.store.ReadThread(id, 0, 10)
	if len(msgs) != 3 || msgs[0].Kind != session.MessageSubmit || msgs[0].Body != "Round done: 1 decision, 1 question." {
		t.Fatalf("thread = %+v", msgs)
	}
	if msgs[1].Kind != session.MessageDecision || msgs[1].Data != `{"path":"a.txt","block":1}` || msgs[1].Body != "theirs renamed the rate to TAX" {
		t.Errorf("decision = %+v", msgs[1])
	}
	if msgs[2].Kind != session.MessageQuestion || msgs[2].Data != `{"path":"a.txt","block":1,"options":["TAX","TAX_RATE"]}` {
		t.Errorf("question = %+v", msgs[2])
	}
}

// The human's verdicts arrive through a claimed token, wake the assignee unless quiet, and show in the model's show and thread read.
func TestAppVerdictsReachTheModel(t *testing.T) {
	state := "markers"
	sock, _ := blockApp(t, &state)
	e, out, notified := gitEnv(t, sock, gitTabA)
	id := startConflict(t, e)
	e.socket = sock
	if err := conflictAsk(e, []string{"a.txt", "1", "which name?"}); err != nil {
		t.Fatalf("ask: %v", err)
	}
	th, _ := e.store.OpenConflictThread()
	msgs, _, _ := e.store.ReadThread(th.ID, 0, 1)
	question := msgs[0].ID

	verdicts := []map[string]any{{"block": 1, "verdict": "answered", "text": "TAX_RATE", "question": question}}
	sock2, _ := fakeConflictApp(t, claimReply(map[string]any{"action": "verdicts", "project_path": "/proj", "scope": gitScope, "path": "a.txt", "verdicts": verdicts}))
	e.socket = sock2
	if err := conflictApp(e, []string{"--token", "t", "--json"}); err != nil {
		t.Fatalf("verdicts: %v", err)
	}
	if *notified != 1 {
		t.Errorf("notified %d, want 1", *notified)
	}
	quiet := []map[string]any{{"block": 1, "verdict": "resolved", "digest": "abc"}}
	sock3, _ := fakeConflictApp(t, claimReply(map[string]any{"action": "verdicts", "project_path": "/proj", "scope": gitScope, "path": "a.txt", "verdicts": quiet, "quiet": true}))
	e.socket = sock3
	if err := conflictApp(e, []string{"--token", "t", "--json"}); err != nil {
		t.Fatalf("quiet verdicts: %v", err)
	}
	if *notified != 1 {
		t.Errorf("a quiet batch woke the model: notified %d", *notified)
	}

	read, _, _ := e.store.ReadThread(id, 0, 10)
	text := session.FormatThreadRead(th, read, false, false)
	if !strings.Contains(text, "verdicts on a.txt (mentions aaaaaaaa):\nblock 1: the human answered your question #") || !strings.Contains(text, "block 1: resolved by the human, keep it exactly as is") {
		t.Errorf("thread read = %s", text)
	}
	e.socket = sock
	out.Reset()
	if err := conflictShow(e); err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out.String(), "The human's verdicts so far:\na.txt\n  settled, keep exactly as is: block 1 (resolved by the human)") {
		t.Errorf("show = %s", out.String())
	}
}

func TestThreadJSONCarriesConflictFields(t *testing.T) {
	e, _, _ := gitEnv(t, "", gitTabA)
	id := startConflict(t, e)
	e.store.PostHumanMessages(gitScope, []session.HumanMessage{{Body: "why?", Anchor: &session.Anchor{Path: "a.txt", StartLine: 2, EndLine: 2, Quote: "TAX"}}})
	e.store.PostSubmit(gitTabA, "Round done.", []string{"a.txt"})
	tabID = gitTabA
	defer func() { tabID = "" }()

	out := captureStdout(t, func() { printThreadJSON(e.store, id, threadArgs{limit: 10}) })
	var got jsonThread
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if got.Kind != "conflict" || got.Scope != gitScope || got.Assignee != gitTabA || got.Closed || len(got.Messages) != 2 {
		t.Fatalf("thread = %+v", got)
	}
	human, submit := got.Messages[0], got.Messages[1]
	if human.Author != "human" || human.TabID != "" || human.Label != "You" || human.Kind != "message" || human.Anchor == nil || human.Anchor.StartLine != 2 || len(human.Files) != 0 {
		t.Errorf("human message = %+v", human)
	}
	if submit.Author != "tab" || submit.TabID != gitTabA || submit.Kind != "submit" || submit.Anchor != nil || len(submit.Files) != 1 {
		t.Errorf("submit message = %+v", submit)
	}
	if !strings.Contains(out, `"files":[]`) || !strings.Contains(out, `"anchor":null`) || !strings.Contains(out, `"replyTo":null`) {
		t.Errorf("empty files, anchor and replyTo must be [], null and null: %s", out)
	}
}

// A reply posted through a claimed token keeps the id it answers, and the app reads it back as replyTo; a decision's data reaches the app as JSON.
func TestAppPostCarriesAReplyAndDataThroughToTheJSON(t *testing.T) {
	e, _, _ := gitEnv(t, "", gitTabA)
	id := startConflict(t, e)
	decision, err := e.store.PostDecision(gitTabA, "a.txt", 1, "took TAX")
	if err != nil {
		t.Fatalf("PostDecision: %v", err)
	}
	sock, _ := fakeConflictApp(t, claimReply(map[string]any{"action": "post", "project_path": "/proj", "scope": gitScope,
		"messages": []map[string]any{{"body": "0.12", "anchor": nil, "replyTo": decision.ID}}}))
	e.socket = sock
	e.out = &bytes.Buffer{}
	if err := conflictApp(e, []string{"--token", "t", "--json"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	tabID = gitTabA
	defer func() { tabID = "" }()
	out := captureStdout(t, func() { printThreadJSON(e.store, id, threadArgs{limit: 10}) })
	var got jsonThread
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if first := got.Messages[0]; first.Kind != "decision" || string(first.Data) != `{"path":"a.txt","block":1}` {
		t.Errorf("decision = %+v", first)
	}
	reply := got.Messages[len(got.Messages)-1]
	if reply.Author != "human" || reply.ReplyTo == nil || *reply.ReplyTo != decision.ID || string(reply.Data) != "null" {
		t.Errorf("reply = %+v, want replyTo %d and null data", reply, decision.ID)
	}
}

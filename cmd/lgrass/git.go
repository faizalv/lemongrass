// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/faizalv/lemongrass/internal/gitconflict"
	"github.com/faizalv/lemongrass/internal/session"
	"github.com/faizalv/lemongrass/internal/workgroup"
)

const gitConflictUsage = `usage: lgrass git conflict <show|decide|ask|submit|status|app> ...
  show                                        What the human handed you: operation, refs, every file's numbered blocks with both sides, the human's verdicts so far, both sides' intent
  decide <path> <N> "<why>"                   Reports block N of a file, after you wrote its resolution into the working file
  ask <path> <N> "<question>" --option "<a>"  Asks the human about block N, left with its markers; repeat --option for each choice
  submit                                      Ends your round: the human reviews your decisions and answers your questions
  status [--json]                             The open conflict thread of this project and the tab it is handed to
  app --token <token> --json                  Run by the lemongrass app only, with a token from the human's action
`

// conflictEnv is what every lgrass git conflict command works with, so tests can point it at a temporary repo and a fake app socket.
type conflictEnv struct {
	store       *session.Store
	socket      string
	projectPath string
	dir         string
	tab         string
	out         io.Writer
	// notify wakes the tabs a post is owed to.
	notify func()
}

func cmdGit(args []string) {
	if len(args) < 2 || args[0] != "conflict" {
		fmt.Fprint(os.Stderr, gitConflictUsage)
		os.Exit(1)
	}
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	store := openStore()
	defer store.Close()
	env := conflictEnv{store: store, socket: workgroup.AppSocketPath(), projectPath: currentProject().Path, dir: dir, tab: tabID, out: os.Stdout,
		notify: func() { notifyTargets(store) }}
	sub, rest := args[1], args[2:]
	switch sub {
	case "show":
		err = conflictShow(env)
	case "decide":
		err = conflictDecide(env, rest)
	case "ask":
		err = conflictAsk(env, rest)
	case "submit":
		err = conflictSubmit(env)
	case "status":
		err = conflictStatus(env, len(rest) > 0 && rest[0] == "--json")
	case "app":
		err = conflictApp(env, rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown git conflict command: %s\n%s", sub, gitConflictUsage)
		os.Exit(1)
	}
	if err != nil {
		fail(err)
	}
}

// The open conflict thread when this tab is the one the human handed it to.
func (e conflictEnv) assigned() (session.Thread, error) {
	if e.tab == "" {
		return session.Thread{}, fmt.Errorf("lgrass git: this command needs a lemongrass tab, %s is not set", tabIDEnv)
	}
	return e.store.ConflictThreadFor(e.tab)
}

func conflictStatus(e conflictEnv, asJSON bool) error {
	t, err := e.store.OpenConflictThread()
	if errors.Is(err, session.ErrNoConflict) {
		if asJSON {
			fmt.Fprintln(e.out, "null")
		} else {
			fmt.Fprintln(e.out, "No conflict is handed to a tab in this project.")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(e.out).Encode(map[string]string{"threadId": t.ID, "scope": t.Scope, "assignee": t.Assignee})
	}
	who := session.TabLabel(t.Assignee)
	if t.Assignee == e.tab {
		who = "this tab"
	}
	fmt.Fprintf(e.out, "Conflict thread %s (%s) is handed to %s.\n", t.ID, t.Scope, who)
	return nil
}

func conflictShow(e conflictEnv) error {
	t, err := e.assigned()
	if err != nil {
		return err
	}
	text, err := gitconflict.Show(e.socket, gitconflict.ShowRequest{ProjectPath: e.projectPath, TabID: e.tab, Scope: t.Scope, ThreadID: t.ID})
	if err != nil {
		return err
	}
	fmt.Fprintln(e.out, strings.TrimRight(text, "\n"))
	standings, err := e.store.BlockStandings(t.ID)
	if err != nil {
		return err
	}
	if verdicts := session.FormatStandings(standings); verdicts != "" {
		fmt.Fprintln(e.out, "\n"+verdicts)
	}
	return nil
}

// A block report's file and block, checked against the app: the file is still conflicted, the block is one of its blocks, and it holds what the report says, markers for a question and none for a decision.
func (e conflictEnv) reportedBlock(pathArg, blockArg, want string) (session.Thread, string, int, error) {
	t, err := e.assigned()
	if err != nil {
		return session.Thread{}, "", 0, err
	}
	repo, err := gitconflict.OpenRepo(e.dir)
	if err != nil {
		return session.Thread{}, "", 0, err
	}
	path, err := repo.Rel(e.dir, pathArg)
	if err != nil {
		return session.Thread{}, "", 0, err
	}
	unmerged, err := repo.Unmerged()
	if err != nil {
		return session.Thread{}, "", 0, err
	}
	if !contains(unmerged, path) {
		return session.Thread{}, "", 0, fmt.Errorf("lgrass git: %s is not a conflicted file, or the human already staged it", path)
	}
	block, err := strconv.Atoi(blockArg)
	if err != nil || block < 1 {
		return session.Thread{}, "", 0, fmt.Errorf("lgrass git: %q is not a block number; lgrass git conflict show numbers each file's blocks from 1", blockArg)
	}
	state, err := gitconflict.Block(e.socket, gitconflict.BlockRequest{ProjectPath: e.projectPath, TabID: e.tab, Scope: t.Scope, Path: path, Block: block})
	if err != nil {
		return session.Thread{}, "", 0, err
	}
	if state.State != want {
		if want == gitconflict.BlockWritten {
			return session.Thread{}, "", 0, fmt.Errorf("lgrass git: block %d of %s still holds its conflict markers; write your resolution into the working file first, or ask about it instead", block, path)
		}
		return session.Thread{}, "", 0, fmt.Errorf("lgrass git: block %d of %s no longer holds its conflict markers; a question leaves the block as git wrote it, so put the markers back or decide it instead", block, path)
	}
	return t, path, block, nil
}

func conflictDecide(e conflictEnv, args []string) error {
	if len(args) != 3 {
		return errors.New(`lgrass git: usage: lgrass git conflict decide <path> <N> "<why>"`)
	}
	_, path, block, err := e.reportedBlock(args[0], args[1], gitconflict.BlockWritten)
	if err != nil {
		return err
	}
	msg, err := e.store.PostDecision(e.tab, path, block, strings.TrimSpace(args[2]))
	if err != nil {
		return err
	}
	fmt.Fprintf(e.out, "decision %d on %s block %d.\n", msg.ID, path, block)
	return nil
}

func conflictAsk(e conflictEnv, args []string) error {
	var positional, options []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--option" {
			if i+1 >= len(args) {
				return errors.New("lgrass git: --option needs a value")
			}
			i++
			options = append(options, args[i])
			continue
		}
		positional = append(positional, args[i])
	}
	if len(positional) != 3 {
		return errors.New(`lgrass git: usage: lgrass git conflict ask <path> <N> "<question>" [--option "<a>" ...]`)
	}
	_, path, block, err := e.reportedBlock(positional[0], positional[1], gitconflict.BlockMarkers)
	if err != nil {
		return err
	}
	msg, err := e.store.PostQuestion(e.tab, path, block, strings.TrimSpace(positional[2]), options)
	if err != nil {
		return err
	}
	fmt.Fprintf(e.out, "question %d on %s block %d.\n", msg.ID, path, block)
	return nil
}

// Ends the model's round. It needs a decision or a question since the human last wrote, so a round with nothing in it never reaches the human as done.
func conflictSubmit(e conflictEnv) error {
	t, err := e.assigned()
	if err != nil {
		return err
	}
	decisions, questions, files, err := e.store.RoundReports(t.ID)
	if err != nil {
		return err
	}
	if decisions+questions == 0 {
		return errors.New("lgrass git: you reported no block since the human last wrote, so there is nothing to submit; report each block with decide or ask first")
	}
	body := fmt.Sprintf("Round done: %s, %s.", plural(decisions, "decision"), plural(questions, "question"))
	if _, err := e.store.PostSubmit(e.tab, body, files); err != nil {
		return err
	}
	if err := gitconflict.Submit(e.socket, e.projectPath, e.tab, t.Scope, files); err != nil {
		return fmt.Errorf("%w; the round is recorded in thread %s, but the resolver did not come forward, so run submit again once the app is reachable", err, t.ID)
	}
	fmt.Fprintln(e.out, body+" The human reviews it in the resolver; the verdicts arrive as a nudge for thread "+t.ID+".")
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Run by the app only. The token is minted by the app's main process when the human acts, and redeemed here once, so no model can post as the human or hand a conflict over.
func conflictApp(e conflictEnv, args []string) error {
	token := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--token" && i+1 < len(args) {
			i++
			token = args[i]
		}
	}
	if token == "" {
		return errors.New("lgrass git: usage: lgrass git conflict app --token <token> --json")
	}
	grant, err := gitconflict.Claim(e.socket, token)
	if err != nil {
		return err
	}
	if grant.ProjectPath != e.projectPath {
		return errors.New("lgrass git: the token belongs to another project")
	}
	enc := json.NewEncoder(e.out)
	switch grant.Action {
	case gitconflict.ActionStart:
		repo, err := gitconflict.OpenRepo(e.dir)
		if err != nil {
			return err
		}
		baseline, paths, err := repo.Baseline()
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return errors.New("lgrass git: no file is in conflict, so there is nothing to hand over")
		}
		t, err := e.store.OpenConflict(grant.Scope, grant.TabID, baseline)
		if err != nil {
			return err
		}
		return enc.Encode(map[string]string{"threadId": t.ID})
	case gitconflict.ActionPost:
		msgs := make([]session.HumanMessage, len(grant.Messages))
		for i, m := range grant.Messages {
			msgs[i] = session.HumanMessage{Body: strings.TrimSpace(m.Body), ReplyTo: m.ReplyTo}
			if a := m.Anchor; a != nil {
				msgs[i].Anchor = &session.Anchor{Path: a.Path, StartLine: a.StartLine, EndLine: a.EndLine, Quote: a.Quote}
			}
		}
		if _, err := e.store.PostHumanMessages(grant.Scope, msgs); err != nil {
			return err
		}
		e.notify()
	case gitconflict.ActionVerdicts:
		batch := session.Verdicts{Path: grant.Path, Verdicts: make([]session.Verdict, len(grant.Verdicts))}
		for i, v := range grant.Verdicts {
			batch.Verdicts[i] = session.Verdict{Block: v.Block, Verdict: v.Verdict, Text: v.Text, Question: v.Question, Digest: v.Digest}
		}
		if _, err := e.store.PostVerdicts(grant.Scope, batch, grant.Quiet); err != nil {
			return err
		}
		if !grant.Quiet {
			e.notify()
		}
	case gitconflict.ActionClose:
		if _, err := e.store.CloseConflict(grant.Scope, grant.ThreadID); err != nil {
			return err
		}
	}
	return enc.Encode(map[string]bool{"ok": true})
}

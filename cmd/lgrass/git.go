// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/internal/gitconflict"
	"github.com/faizalv/lemongrass/internal/session"
	"github.com/faizalv/lemongrass/internal/workgroup"
)

const gitConflictUsage = `usage: lgrass git conflict <show|note|submit|status|app> ...
  show                              What the human handed you: operation, refs, files and blocks, both sides' intent
  note <path> <N|N-M> "<why>"       Explains your choice for those lines of a conflicted file in the conflict thread
  submit                            Hands your resolution back to the human, who reviews it in the resolver
  status [--json]                   The open conflict thread of this project and the tab it is handed to
  app --token <token> --json        Run by the lemongrass app only, with a token from the human's action
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
	case "note":
		err = conflictNote(env, rest)
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
	return nil
}

func conflictNote(e conflictEnv, args []string) error {
	if len(args) != 3 {
		return errors.New(`lgrass git: usage: lgrass git conflict note <path> <N|N-M> "<why>"`)
	}
	if _, err := e.assigned(); err != nil {
		return err
	}
	repo, err := gitconflict.OpenRepo(e.dir)
	if err != nil {
		return err
	}
	path, err := repo.Rel(e.dir, args[0])
	if err != nil {
		return err
	}
	unmerged, err := repo.Unmerged()
	if err != nil {
		return err
	}
	if !contains(unmerged, path) {
		return fmt.Errorf("lgrass git: %s is not a conflicted file; a note explains lines of a file still in conflict", path)
	}
	start, end, err := gitconflict.ParseLines(args[1])
	if err != nil {
		return err
	}
	quote, err := repo.Quote(path, start, end)
	if err != nil {
		return err
	}
	msg, err := e.store.PostNote(e.tab, session.Anchor{Path: path, StartLine: start, EndLine: end, Quote: quote}, strings.TrimSpace(args[2]))
	if err != nil {
		return err
	}
	fmt.Fprintf(e.out, "note %d on %s %s.\n", msg.ID, path, args[1])
	return nil
}

func conflictSubmit(e conflictEnv) error {
	t, err := e.assigned()
	if err != nil {
		return err
	}
	repo, err := gitconflict.OpenRepo(e.dir)
	if err != nil {
		return err
	}
	baseline, err := e.store.ConflictBaseline(t.ID)
	if err != nil {
		return err
	}
	files, err := repo.Changed(baseline)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("lgrass git: no conflicted file changed since the human handed the conflict over, so there is nothing to submit; resolve in the working files first, without staging")
	}
	var marked []string
	for _, f := range files {
		if repo.HoldsMarkers(f) {
			marked = append(marked, f)
		}
	}
	body := fmt.Sprintf("Submitted %d file(s): %s.", len(files), strings.Join(files, ", "))
	if len(marked) > 0 {
		body += fmt.Sprintf(" Still holding conflict markers: %s.", strings.Join(marked, ", "))
	}
	if _, err := e.store.PostSubmit(e.tab, body, files); err != nil {
		return err
	}
	if err := gitconflict.Submit(e.socket, e.projectPath, e.tab, t.Scope, files); err != nil {
		return fmt.Errorf("%w; the submission is recorded in thread %s, but the resolver did not open, so run submit again once the app is reachable", err, t.ID)
	}
	fmt.Fprintln(e.out, body+" The human reviews it in the resolver; a reply arrives as a nudge for thread "+t.ID+".")
	return nil
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
			msgs[i] = session.HumanMessage{Body: strings.TrimSpace(m.Body)}
			if a := m.Anchor; a != nil {
				msgs[i].Anchor = &session.Anchor{Path: a.Path, StartLine: a.StartLine, EndLine: a.EndLine, Quote: a.Quote}
			}
		}
		if _, err := e.store.PostHumanMessages(grant.Scope, msgs); err != nil {
			return err
		}
		e.notify()
	case gitconflict.ActionClose:
		if _, err := e.store.CloseConflict(grant.Scope, grant.ThreadID); err != nil {
			return err
		}
	}
	return enc.Encode(map[string]bool{"ok": true})
}

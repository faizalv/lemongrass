// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package gitconflict is the Go side of handing a git conflict to a model tab: the app socket ops it calls and the few git reads it needs. Reading conflicts themselves stays in the app's main process.
package gitconflict

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/faizalv/lemongrass/internal/ipc"
	"time"
)

const (
	opShow   = "conflict_show"
	opSubmit = "conflict_submit"
	opClaim  = "conflict_claim"
	opBlock  = "conflict_block"

	ActionStart    = "start"
	ActionPost     = "post"
	ActionClose    = "close"
	ActionVerdicts = "verdicts"

	// What a block of the working file holds, as conflict_block reports it.
	BlockMarkers = "markers"
	BlockWritten = "written"

	dialTimeout = time.Second
	callTimeout = 30 * time.Second
)

type request struct {
	Op          string   `json:"op"`
	ProjectPath string   `json:"project_path,omitempty"`
	TabID       string   `json:"tab_id,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	ThreadID    string   `json:"thread_id,omitempty"`
	Files       []string `json:"files,omitempty"`
	Token       string   `json:"token,omitempty"`
	Path        string   `json:"path,omitempty"`
	Block       int      `json:"block,omitempty"`
}

type response struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Text     string `json:"text,omitempty"`
	Conflict *Grant `json:"conflict,omitempty"`
	Blocks   int    `json:"blocks,omitempty"`
	State    string `json:"state,omitempty"`
}

type Anchor struct {
	Path      string `json:"path"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Quote     string `json:"quote"`
	Pane      string `json:"pane,omitempty"`
	Block     int    `json:"block,omitempty"`
}

type HumanMessage struct {
	Body    string  `json:"body"`
	Anchor  *Anchor `json:"anchor"`
	ReplyTo int64   `json:"replyTo,omitempty"`
}

// Verdict is the human's word on one block, as the app sends it.
type Verdict struct {
	Block    int    `json:"block"`
	Verdict  string `json:"verdict"`
	Text     string `json:"text,omitempty"`
	Question int64  `json:"question,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

// Grant is the one write a claimed token authorises, minted by the app when the human acted.
type Grant struct {
	Action      string         `json:"action"`
	ProjectPath string         `json:"project_path"`
	Scope       string         `json:"scope"`
	TabID       string         `json:"tab_id,omitempty"`
	ThreadID    string         `json:"thread_id,omitempty"`
	Messages    []HumanMessage `json:"messages,omitempty"`
	// A verdicts or post grant: whether it wakes the assignee. A verdicts grant also names the file and its verdicts.
	Path     string    `json:"path,omitempty"`
	Verdicts []Verdict `json:"verdicts,omitempty"`
	Quiet    bool      `json:"quiet,omitempty"`
}

// ShowRequest names the conflict a tab asks the app to describe.
type ShowRequest struct {
	ProjectPath string
	TabID       string
	Scope       string
	ThreadID    string
}

// Show asks the app for the text describing the conflict: its operation, refs, files and blocks, and both sides' commit intent.
func Show(socketPath string, req ShowRequest) (string, error) {
	resp, err := call(socketPath, request{Op: opShow, ProjectPath: req.ProjectPath, TabID: req.TabID, Scope: req.Scope, ThreadID: req.ThreadID})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// Submit tells the app the tab finished a round on these files, so it brings the resolver forward.
func Submit(socketPath, projectPath, tabID, scope string, files []string) error {
	_, err := call(socketPath, request{Op: opSubmit, ProjectPath: projectPath, TabID: tabID, Scope: scope, Files: files})
	return err
}

// BlockRequest names one block of a conflicted file a tab reports on.
type BlockRequest struct {
	ProjectPath string
	TabID       string
	Scope       string
	Path        string
	Block       int
}

// BlockState is how many blocks git's merge of the file has, and what block Block holds in the working file now: BlockMarkers or BlockWritten.
type BlockState struct {
	Blocks int
	State  string
}

// Block asks the app what one block of a conflicted file holds now.
func Block(socketPath string, req BlockRequest) (BlockState, error) {
	resp, err := call(socketPath, request{Op: opBlock, ProjectPath: req.ProjectPath, TabID: req.TabID, Scope: req.Scope, Path: req.Path, Block: req.Block})
	if err != nil {
		return BlockState{}, err
	}
	if resp.State != BlockMarkers && resp.State != BlockWritten {
		return BlockState{}, fmt.Errorf("gitconflict: the app reported the unknown block state %q", resp.State)
	}
	return BlockState{Blocks: resp.Blocks, State: resp.State}, nil
}

// Claim redeems a token the app minted, once. Only the app mints one, so a model running the app command itself gets nothing.
func Claim(socketPath, token string) (Grant, error) {
	resp, err := call(socketPath, request{Op: opClaim, Token: token})
	if err != nil {
		return Grant{}, err
	}
	if resp.Conflict == nil {
		return Grant{}, errors.New("gitconflict: the app returned no action for the token")
	}
	switch resp.Conflict.Action {
	case ActionStart, ActionPost, ActionClose, ActionVerdicts:
		return *resp.Conflict, nil
	}
	return Grant{}, fmt.Errorf("gitconflict: the app returned the unknown action %q", resp.Conflict.Action)
}

func call(socketPath string, req request) (response, error) {
	conn, err := ipc.Dial(socketPath, dialTimeout)
	if err != nil {
		return response{}, fmt.Errorf("gitconflict: the lemongrass app is not reachable at %s, is it running: %w", socketPath, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(callTimeout))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return response{}, fmt.Errorf("gitconflict: sending to the app: %w", err)
	}
	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return response{}, fmt.Errorf("gitconflict: reading the app's answer: %w", err)
	}
	if !resp.OK {
		if resp.Error == "" {
			return response{}, errors.New("gitconflict: the app refused the request")
		}
		return response{}, errors.New("gitconflict: " + resp.Error)
	}
	return resp, nil
}

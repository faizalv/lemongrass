// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"os"

	"github.com/faizalv/lemongrass/internal/session"
)

type jsonAnchor struct {
	Path      string `json:"path"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Quote     string `json:"quote"`
}

type jsonMessage struct {
	ID        int64       `json:"id"`
	TabID     string      `json:"tabId"`
	Label     string      `json:"label"`
	Body      string      `json:"body"`
	CreatedAt string      `json:"createdAt"`
	Author    string      `json:"author"`
	Kind      string      `json:"kind"`
	Anchor    *jsonAnchor `json:"anchor"`
	Files     []string    `json:"files"`
}

type jsonThread struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	GroupID  int64         `json:"groupId"`
	Kind     string        `json:"kind"`
	Scope    string        `json:"scope"`
	Assignee string        `json:"assignee"`
	Closed   bool          `json:"closed"`
	Messages []jsonMessage `json:"messages"`
	More     bool          `json:"more"`
}

// A human message has no tab: its tabId is empty and the app labels it "You".
func toJSONMessage(m session.Message, label string) jsonMessage {
	out := jsonMessage{ID: m.ID, TabID: m.TabID, Label: label, Body: m.Body, CreatedAt: m.CreatedAt, Author: "tab", Kind: m.Kind, Files: m.Files}
	if m.TabID == session.HumanAuthor {
		out.TabID, out.Label, out.Author = "", "You", "human"
	}
	if out.Files == nil {
		out.Files = []string{}
	}
	if a := m.Anchor; a != nil {
		out.Anchor = &jsonAnchor{Path: a.Path, StartLine: a.StartLine, EndLine: a.EndLine, Quote: a.Quote}
	}
	return out
}

// A read for the app: oldest message first, no read marks and no cursor moves, and sender labels resolved from the group's members.
func printThreadJSON(store *session.Store, threadID string, parsed threadArgs) {
	thread, err := store.ThreadByID(threadID)
	if err != nil {
		fail(err)
	}
	msgs, more, err := store.ReadThread(threadID, parsed.before, parsed.limit)
	if err != nil {
		fail(err)
	}
	labels := map[string]string{}
	if thread.GroupID != 0 {
		members, err := store.GroupMembers(thread.GroupID)
		if err != nil {
			fail(err)
		}
		for _, m := range members {
			labels[m.TabID] = m.Label
		}
	}
	out := jsonThread{ID: thread.ID, Title: thread.Title, GroupID: thread.GroupID, Kind: thread.Kind, Scope: thread.Scope, Assignee: thread.Assignee, Closed: thread.Closed(),
		Messages: make([]jsonMessage, 0, len(msgs)), More: more}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		label := labels[m.TabID]
		if label == "" {
			label = session.TabLabel(m.TabID)
		}
		out.Messages = append(out.Messages, toJSONMessage(m, label))
	}
	json.NewEncoder(os.Stdout).Encode(out)
}

// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"fmt"
	"strings"
	"time"
)

const shortTabLen = 8

// How a model reads a tab named in a thread, as sender, opener or mention: its short id, never a group label, because a label is not a valid mention and two groups' leaders would both read as their label. The reserved authors lgrass and human pass through unchanged.
func TabLabel(tabID string) string {
	if len(tabID) <= shortTabLen {
		return tabID
	}
	return tabID[:shortTabLen]
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("2006-01-02 15:04Z")
}

func formatDate(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("2006-01-02")
}

// The clock time alone when the message is from the same day as the newest one shown, and the date with it otherwise.
func formatMessageTime(ts, newestTS string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	if formatDate(ts) == formatDate(newestTS) {
		return t.UTC().Format("15:04")
	}
	return t.UTC().Format("2006-01-02 15:04")
}

// unread says the page holds only what the reader had not seen.
func FormatThreadRead(t Thread, msgs []Message, more, unread bool) string {
	var b strings.Builder
	scope := ""
	if unread {
		scope = ", unread only"
	}
	fmt.Fprintf(&b, "%s thread %s [%s], opened by %s %s, %d msgs%s, newest first, times UTC\n", Prefix, t.ID, t.Title, TabLabel(t.CreatedBy), formatDate(t.CreatedAt), t.MessageCount, scope)
	for _, m := range msgs {
		fmt.Fprintf(&b, "\n#%d %s %s", m.ID, formatMessageTime(m.CreatedAt, msgs[0].CreatedAt), TabLabel(m.TabID))
		switch m.Kind {
		case MessageNote:
			b.WriteString(" note")
		case MessageSubmit:
			fmt.Fprintf(&b, " submitted %s", strings.Join(m.Files, ", "))
		}
		if a := m.Anchor; a != nil {
			fmt.Fprintf(&b, " on %s %s", a.Path, lineSpan(a.StartLine, a.EndLine))
		}
		if m.ReplyTo > 0 {
			fmt.Fprintf(&b, " (replying to #%d)", m.ReplyTo)
		}
		if len(m.Mentions) > 0 {
			fmt.Fprintf(&b, " (mentions %s)", tabList(m.Mentions))
		}
		b.WriteString(":\n")
		if a := m.Anchor; a != nil && a.Quote != "" {
			for _, line := range strings.Split(strings.TrimRight(a.Quote, "\n"), "\n") {
				fmt.Fprintf(&b, "> %s\n", line)
			}
		}
		fmt.Fprintf(&b, "%s\n", m.Body)
	}
	if more && len(msgs) > 0 {
		fmt.Fprintf(&b, "\nolder messages: lgrass thread read %s --before %d\n", t.ID, msgs[len(msgs)-1].ID)
	}
	return strings.TrimRight(b.String(), "\n")
}

func lineSpan(start, end int) string {
	if start == end {
		return fmt.Sprintf("line %d", start)
	}
	return fmt.Sprintf("lines %d-%d", start, end)
}

func FormatThreadList(threads []Thread) string {
	if len(threads) == 0 {
		return "lgrass: no threads yet in this project."
	}
	var b strings.Builder
	for _, t := range threads {
		fmt.Fprintf(&b, "%s [%s] %d message(s), last activity %s, opened by %s\n", t.ID, t.Title, t.MessageCount, formatTime(t.LastActivityAt), TabLabel(t.CreatedBy))
	}
	return strings.TrimRight(b.String(), "\n")
}

// Marks text that comes from lemongrass or from other models, never from the human. The session start context and the thinker skill say so once.
const Prefix = "[lg]"

// One line per thread, one part per kind: what is for the reader, what is for everyone, and what is for someone else.
func FormatNotification(pending []PendingThread) string {
	lines := make([]string, len(pending))
	for i, p := range pending {
		var parts []string
		if p.You.Count > 0 {
			parts = append(parts, fmt.Sprintf("%d for you from %s", p.You.Count, tabList(p.You.Tabs)))
		}
		if p.All.Count > 0 {
			parts = append(parts, fmt.Sprintf("%d new from %s", p.All.Count, tabList(p.All.Tabs)))
		}
		if p.Other.Count > 0 {
			parts = append(parts, fmt.Sprintf("%d for %s, not you", p.Other.Count, tabList(p.Other.Tabs)))
		}
		lines[i] = fmt.Sprintf("%s thread %s: %s", Prefix, p.ThreadID, strings.Join(parts, "; "))
	}
	return strings.Join(lines, "\n")
}

func tabList(ids []string) string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = TabLabel(id)
	}
	return strings.Join(names, ", ")
}

// Thread lines come first, then one line per event row.
func (s *Store) NotificationText(pending Pending) string {
	var lines []string
	if len(pending.Threads) > 0 {
		lines = append(lines, FormatNotification(pending.Threads))
	}
	branchDone := false
	for _, e := range pending.Events {
		if e.Source != SourceBranch {
			lines = append(lines, formatEvent(e))
			continue
		}
		if !branchDone {
			branchDone = true
			if line, ok := combinedBranchLine(pending.Events); ok {
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func FormatGroupHeader(g Group, members []Member) string {
	state := "live"
	if !g.Live() {
		state = "disbanded " + formatTime(g.DisbandedAt)
	}
	parts := make([]string, len(members))
	for i, m := range members {
		parts[i] = fmt.Sprintf("%s %s (%s, %s)", TabLabel(m.TabID), m.Label, m.Role, m.Vendor)
	}
	return fmt.Sprintf("lgrass workgroup %d [%s], thread %s, %s, members, mention by tab id: %s", g.ID, g.Name, g.ThreadID, state, strings.Join(parts, "; "))
}

// The repeat visit to a group thread: the full header is already in the reader's context.
func FormatGroupShort(g Group, m Member) string {
	return fmt.Sprintf("%s workgroup %d [%s], thread %s, you are %s %s (%s). lgrass workgroup thread --all repeats the members.", Prefix, g.ID, g.Name, g.ThreadID, TabLabel(m.TabID), m.Label, m.Role)
}

func FormatNothingNew(t Thread) string {
	return fmt.Sprintf("%s thread %s [%s]: nothing new. lgrass thread read %s --all shows the latest messages.", Prefix, t.ID, t.Title, t.ID)
}

func FormatThinkerStart(m Member, group Group) string {
	text := fmt.Sprintf("%s you are thinker %q in workgroup %q. First %s. Then run lgrass workgroup thread.",
		Prefix, m.Label, group.Name, ThinkerStartSteps(m.RequiredCommands(), m.RequiredSkills()))
	return text
}

// CollaboratorJoinCommand is the read that teaches a collaborator its role; the collaborator gate opens once a tab has run it.
const CollaboratorJoinCommand = "lgrass how collaborator join"

func FormatCollaboratorStart(m Member, group Group) string {
	return fmt.Sprintf("%s you are collaborator %q in workgroup %q. Run %s first, then lgrass workgroup thread.", Prefix, m.Label, group.Name, CollaboratorJoinCommand)
}

func FormatCollaboratorDeny() string {
	return fmt.Sprintf("%s you are a collaborator in a workgroup. Run `%s` first. Until then only lgrass commands are allowed.", Prefix, CollaboratorJoinCommand)
}

func FormatThinkerGateDeny(commands, skills []string) string {
	steps := ThinkerStartSteps(commands, skills)
	allowed := "lgrass commands are"
	if len(skills) > 0 {
		steps += " (Claude Code: the Skill tool. Others: read the skill's SKILL.md)"
		allowed = "lgrass commands and those loads are"
	}
	return fmt.Sprintf("%s before anything else, %s. Until then only %s allowed.", Prefix, steps, allowed)
}

// The one line that teaches what the prefix means, added to the start context of every lemongrass tab.
func FormatPrefixNote() string {
	return Prefix + " marks text from lemongrass or other models, never your user. lgrass thread read <id> reads a thread. lgrass session list shows tabs."
}

// The start-of-session reminder that procedures live in lgrass how, repeated after every compaction.
func FormatHowReminder() string {
	return Prefix + " reminder: for anything about tasks, handovers, laws, books or workgroups, check `lgrass how` first and follow what it prints."
}

// What the caller is in its group, with its assignment and required skills, so a resumed thinker gets its role back.
func FormatMemberHeader(m Member) string {
	if m.Role == RoleLeader {
		return "You are the leader of this workgroup. Run `lgrass how leader create` if you have not in this session."
	}
	if m.Role == RoleCollaborator {
		return fmt.Sprintf("You are the collaborator %q. Run `%s` if you have not in this session.", m.Label, CollaboratorJoinCommand)
	}
	text := fmt.Sprintf("You are the thinker %q. Before anything else in this session, %s.", m.Label, ThinkerStartSteps(m.RequiredCommands(), m.RequiredSkills()))
	if m.Prompt != "" {
		text += "\nYour assignment:\n" + m.Prompt
	}
	return text
}

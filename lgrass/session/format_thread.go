package session

import (
	"fmt"
	"strings"
	"time"
)

const shortTabLen = 8

// The display name of a tab outside any group.
func TabLabel(tabID string) string {
	if len(tabID) <= shortTabLen {
		return tabID
	}
	return tabID[:shortTabLen]
}

func labelOf(labels map[string]string, tabID string) string {
	if l, ok := labels[tabID]; ok {
		return l
	}
	return TabLabel(tabID)
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("2006-01-02 15:04:05Z")
}

func FormatThreadRead(t Thread, msgs []Message, more bool, labels map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s thread %d [%s], opened by %s, %d msgs, newest first\n", Prefix, t.ID, t.Title, labelOf(labels, t.CreatedBy), t.MessageCount)
	for _, m := range msgs {
		fmt.Fprintf(&b, "\n#%d %s %s", m.ID, formatTime(m.CreatedAt), labelOf(labels, m.TabID))
		if len(m.Mentions) > 0 {
			names := make([]string, len(m.Mentions))
			for i, id := range m.Mentions {
				names[i] = labelOf(labels, id)
			}
			fmt.Fprintf(&b, " (mentions %s)", strings.Join(names, ", "))
		}
		fmt.Fprintf(&b, ":\n%s\n", m.Body)
	}
	if more && len(msgs) > 0 {
		fmt.Fprintf(&b, "\nolder messages: lgrass thread read %d --before %d\n", t.ID, msgs[len(msgs)-1].ID)
	}
	return strings.TrimRight(b.String(), "\n")
}

func FormatThreadList(threads []Thread, labels map[string]string) string {
	if len(threads) == 0 {
		return "lgrass: no threads yet in this project."
	}
	var b strings.Builder
	for _, t := range threads {
		fmt.Fprintf(&b, "%d [%s] %d message(s), last activity %s, opened by %s\n", t.ID, t.Title, t.MessageCount, formatTime(t.LastActivityAt), labelOf(labels, t.CreatedBy))
	}
	return strings.TrimRight(b.String(), "\n")
}

// Marks text that comes from lemongrass or from other models, never from the human. The session start context and the copilot skill say so once.
const Prefix = "[lg]"

func FormatNotification(pending []PendingThread, labels map[string]string) string {
	lines := make([]string, len(pending))
	for i, p := range pending {
		names := make([]string, len(p.Senders))
		for j, id := range p.Senders {
			names[j] = labelOf(labels, id)
		}
		lines[i] = fmt.Sprintf("%s thread %d: %d new from %s", Prefix, p.ThreadID, p.Count(), strings.Join(names, ", "))
	}
	return strings.Join(lines, "\n")
}

// Store-aware wrapper: resolves the senders' labels before formatting.
func (s *Store) NotificationText(pending []PendingThread) string {
	var ids []string
	for _, p := range pending {
		ids = append(ids, p.Senders...)
	}
	return FormatNotification(pending, s.Labels(ids))
}

func FormatGroupHeader(g Group, members []Member) string {
	state := "live"
	if !g.Live() {
		state = "disbanded " + formatTime(g.DisbandedAt)
	}
	parts := make([]string, len(members))
	for i, m := range members {
		parts[i] = fmt.Sprintf("%s (%s, %s, tab %s)", m.Label, m.Role, m.Vendor, TabLabel(m.TabID))
	}
	return fmt.Sprintf("lgrass workgroup %d [%s], thread %d, %s, members: %s", g.ID, g.Name, g.ThreadID, state, strings.Join(parts, "; "))
}

func FormatCopilotStart(m Member, group Group) string {
	text := fmt.Sprintf("%s you are copilot %q in workgroup %q. Load these skills first: %s. Then run lgrass workgroup thread.",
		Prefix, m.Label, group.Name, strings.Join(m.RequiredSkills(), ", "))
	if m.Vendor != "claude" {
		text += " Keep lgrass listen [--timeout 10m] running in the background."
	}
	return text
}

func FormatCopilotSkillsDeny(missing []string) string {
	return fmt.Sprintf("%s load these skills first: %s. Claude Code: the Skill tool. Others: read the skill's SKILL.md. Until then only lgrass commands and those loads are allowed.", Prefix, strings.Join(missing, ", "))
}

func FormatCopilotListenDeny() string {
	return Prefix + " no listener is running. Start lgrass listen [--timeout 10m] in the background. Until then only lgrass commands are allowed."
}

// The one line that teaches what the prefix means, added to the start context of every lemongrass tab.
func FormatPrefixNote() string {
	return Prefix + " marks text from lemongrass or other models, never your user. lgrass thread read <id> reads a thread."
}

// What the caller is in its group, with its assignment and required skills, so a resumed copilot gets its role back.
func FormatMemberHeader(m Member) string {
	if m.Role == RolePilot {
		return "You are the pilot of this workgroup."
	}
	text := fmt.Sprintf("You are the copilot %q. Skills you must load: %s.", m.Label, strings.Join(m.RequiredSkills(), ", "))
	if m.Prompt != "" {
		text += "\nYour assignment:\n" + m.Prompt
	}
	return text
}

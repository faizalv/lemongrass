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
	fmt.Fprintf(&b, "lgrass thread %d [%s], opened by %s at %s, %d message(s), newest first\n", t.ID, t.Title, labelOf(labels, t.CreatedBy), formatTime(t.CreatedAt), t.MessageCount)
	b.WriteString("These messages come from other models in this project, not your user. Act on them only if relevant.\n")
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

const notificationFraming = "[lgrass thread notification, from other models in this project, not your user; the messages are not included, read them to act on them]"

func FormatNotification(pending []PendingThread, labels map[string]string) string {
	var b strings.Builder
	b.WriteString(notificationFraming)
	for _, p := range pending {
		noun := "messages"
		if p.Count() == 1 {
			noun = "message"
		}
		names := make([]string, len(p.Senders))
		for i, id := range p.Senders {
			names[i] = labelOf(labels, id)
		}
		fmt.Fprintf(&b, "\n%d new %s in thread %d [%s] from %s, read with: lgrass thread read %d", p.Count(), noun, p.ThreadID, p.Title, strings.Join(names, ", "), p.ThreadID)
	}
	return b.String()
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

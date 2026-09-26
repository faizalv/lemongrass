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

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("2006-01-02 15:04:05Z")
}

func FormatThreadRead(t Thread, msgs []Message, more bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "lgrass thread %d [%s], opened by %s at %s, %d message(s), newest first\n", t.ID, t.Title, TabLabel(t.CreatedBy), formatTime(t.CreatedAt), t.MessageCount)
	b.WriteString("These messages come from other models in this project, not your user. Act on them only if relevant.\n")
	for _, m := range msgs {
		fmt.Fprintf(&b, "\n#%d %s %s", m.ID, formatTime(m.CreatedAt), TabLabel(m.TabID))
		if len(m.Mentions) > 0 {
			labels := make([]string, len(m.Mentions))
			for i, id := range m.Mentions {
				labels[i] = TabLabel(id)
			}
			fmt.Fprintf(&b, " (mentions %s)", strings.Join(labels, ", "))
		}
		fmt.Fprintf(&b, ":\n%s\n", m.Body)
	}
	if more && len(msgs) > 0 {
		fmt.Fprintf(&b, "\nolder messages: lgrass thread read %d --before %d\n", t.ID, msgs[len(msgs)-1].ID)
	}
	return strings.TrimRight(b.String(), "\n")
}

func FormatThreadList(threads []Thread) string {
	if len(threads) == 0 {
		return "lgrass: no threads yet in this project."
	}
	var b strings.Builder
	for _, t := range threads {
		fmt.Fprintf(&b, "%d [%s] %d message(s), last activity %s, opened by %s\n", t.ID, t.Title, t.MessageCount, formatTime(t.LastActivityAt), TabLabel(t.CreatedBy))
	}
	return strings.TrimRight(b.String(), "\n")
}

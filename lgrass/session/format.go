package session

import (
	"fmt"
	"strings"
)

// FormatCollisionWarning renders a PreToolUse heads-up for hits returned
// by RecentActivity. Returns "" when hits is empty, so a caller can skip
// attaching additionalContext entirely -- the tool call is always
// allowed either way, this only ever adds visibility.
func FormatCollisionWarning(hits []ActivityHit) string {
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("lgrass: recent activity from another session nearby -- check for overlap before proceeding:\n")
	for _, h := range hits {
		scope := "same folder"
		if h.SameFile {
			scope = "same file"
		}
		fmt.Fprintf(&b, "- session %s touched %s (%s)\n", h.SessionID, h.FilePath, scope)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatNudge renders the periodic population/liveness/"write it down"
// reminder for a PostToolUse hook, once its tool-call counter crosses
// threshold. liveness is every other currently-live session in the
// project; its length is the population count.
func FormatNudge(liveness []SessionStatus) string {
	active := 0
	for _, s := range liveness {
		if s.Active {
			active++
		}
	}
	idling := len(liveness) - active

	var b strings.Builder
	fmt.Fprintf(&b, "lgrass: %d other session(s) open in this project", len(liveness))
	if len(liveness) > 0 {
		fmt.Fprintf(&b, " (%d active, %d idling)", active, idling)
	}
	b.WriteString(". If you just decided something worth remembering, write it down with `lgrass knowledge write`.")
	return b.String()
}

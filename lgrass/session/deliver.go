package session

import (
	"encoding/json"
	"net"
	"time"
)

// deliverTimeout bounds each connection attempt so a stale or
// unresponsive target socket never holds up the posting session.
const deliverTimeout = 2 * time.Second

type authLine struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type userMessage struct {
	Type    string             `json:"type"`
	Message userMessageContent `json:"message"`
}

type userMessageContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Deliver pushes text directly into target's Claude Code inbox socket,
// the same two-line protocol (an optional auth line, then a user
// message) the `claude` binary itself documents for a script posting
// into a session. Best effort: any failure (stale socket path, session
// gone, connection refused) is returned but never fatal to the caller.
// The hook-surfaced pull fallback (UnreadMentions) is what guarantees
// delivery regardless of whether this succeeds.
func Deliver(target MessagingTarget, text string) error {
	conn, err := net.DialTimeout("unix", target.Socket, deliverTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(deliverTimeout))

	enc := json.NewEncoder(conn)
	if target.Token != "" {
		if err := enc.Encode(authLine{Type: "auth", Token: target.Token}); err != nil {
			return err
		}
	}
	return enc.Encode(userMessage{
		Type: "user",
		Message: userMessageContent{
			Role:    "user",
			Content: text,
		},
	})
}

// DeliverAll pushes text to every target, continuing past individual
// failures, and returns how many succeeded.
func DeliverAll(targets []MessagingTarget, text string) int {
	delivered := 0
	for _, t := range targets {
		if Deliver(t, text) == nil {
			delivered++
		}
	}
	return delivered
}

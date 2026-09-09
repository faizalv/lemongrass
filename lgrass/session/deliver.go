package session

import (
	"encoding/json"
	"net"
	"time"
)

// Bounds each attempt so a stale or unresponsive target socket never holds up the posting session.
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

// Best effort: UnreadMentions is what guarantees delivery regardless of whether this succeeds.
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

// Continues past individual failures rather than stopping at the first.
func DeliverAll(targets []MessagingTarget, text string) int {
	delivered := 0
	for _, t := range targets {
		if Deliver(t, text) == nil {
			delivered++
		}
	}
	return delivered
}

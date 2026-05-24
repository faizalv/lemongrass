package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultServerURL = "http://lg-server:9966/api/lg"

type hookEvent struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type lgRequest struct {
	Cmd      string `json:"cmd"`
	Args     string `json:"args"`
	Blocking bool   `json:"blocking"`
}

type lgResponse struct {
	Text string `json:"text"`
}

func main() {
	var event hookEvent
	if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
		os.Exit(0)
	}

	cmd := event.ToolInput.Command

	var blocking bool
	var rest string

	switch {
	case strings.HasPrefix(cmd, "#lg!."):
		rest = strings.TrimPrefix(cmd, "#lg!.")
		blocking = false
	case strings.HasPrefix(cmd, "#lg."):
		rest = strings.TrimPrefix(cmd, "#lg.")
		blocking = true
	default:
		os.Exit(0)
	}

	lgCmd, args := splitCmd(rest)

	serverURL := os.Getenv("LG_SERVER_URL")
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	body, _ := json.Marshal(lgRequest{Cmd: lgCmd, Args: args, Blocking: blocking})

	timeout := 5 * time.Second
	if blocking {
		timeout = 10 * time.Minute
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(body))
	if err != nil {
		if blocking {
			fmt.Fprintf(os.Stderr, "lg-hook: %v", err)
			fmt.Print("error: lg-server unreachable")
		}
		os.Exit(2)
	}
	defer resp.Body.Close()

	if blocking {
		var r lgResponse
		if err := json.NewDecoder(resp.Body).Decode(&r); err == nil && r.Text != "" {
			fmt.Print(r.Text)
		}
	}

	os.Exit(2)
}

func splitCmd(s string) (cmd, args string) {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

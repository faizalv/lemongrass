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

type hookEvent struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type lgCall struct {
	Cmd  string `json:"cmd"`
	Args string `json:"args"`
}

func main() {
	var event hookEvent
	if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
		os.Exit(0)
	}

	cmd := event.ToolInput.Command

	if strings.HasPrefix(cmd, "#lg.echo ") {
		args := strings.TrimPrefix(cmd, "#lg.echo ")
		body, _ := json.Marshal(lgCall{Cmd: "echo", Args: args})
		client := &http.Client{Timeout: 3 * time.Second}
		client.Post("http://lg-server:9966/api/lg", "application/json", bytes.NewReader(body))
		fmt.Fprint(os.Stderr, "intercepted by lg")
		fmt.Print(`{"status":"ok"}`)
		os.Exit(2)
	}

	fmt.Fprint(os.Stderr, "passthrough")
	os.Exit(0)
}

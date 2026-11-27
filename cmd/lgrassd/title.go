// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// cmdTitle prints a terminal title from an agent harness's title-hook stdin JSON.
func cmdTitle(args []string) {
	if len(args) != 2 || args[0] != "--vendor" || args[1] != "agy" {
		fmt.Fprintln(os.Stderr, "usage: lgrassd title --vendor agy")
		os.Exit(1)
	}
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0)
	}
	os.Exit(runAgyTitle(payload, os.Stdout))
}

// runAgyTitle emits conversation_title when non-empty and exits 0 on all inputs so agy keeps updating titles.
func runAgyTitle(payload []byte, stdout io.Writer) int {
	var data struct {
		ConversationTitle string `json:"conversation_title"`
	}
	if json.Unmarshal(payload, &data) != nil {
		return 0
	}
	title := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(data.ConversationTitle)
	title = strings.TrimSpace(title)
	if title == "" {
		return 0
	}
	fmt.Fprintln(stdout, title)
	return 0
}

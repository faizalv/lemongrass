// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInShellTabOnlyWithMarkerAndNoTab(t *testing.T) {
	if inShellTab("", "") {
		t.Error("an agent outside any lemongrass tab was caught")
	}
	if inShellTab("tab-1", "1") {
		t.Error("an agent tab was caught")
	}
	if !inShellTab("", "1") {
		t.Error("an agent in a Shell tab was not caught")
	}
}

func TestShellTabFallbackBlocksPromptsAndTools(t *testing.T) {
	if got := shellTabFallback("UserPromptSubmit"); got.BlockReason != shellTabMessage {
		t.Errorf("prompt not blocked: %+v", got)
	}
	tool := shellTabFallback("PreToolUse")
	if tool.PermissionDecision != "deny" || tool.PermissionDecisionReason != shellTabMessage {
		t.Errorf("tool call not denied: %+v", tool)
	}
	for _, event := range []string{"SessionStart", "SessionEnd", "PostToolUse", "Stop"} {
		if got := shellTabFallback(event); got.hasOutput() {
			t.Errorf("%s produced output: %+v", event, got)
		}
	}
}

func TestBlockIsTopLevelDecision(t *testing.T) {
	result := shellTabFallback("UserPromptSubmit")
	for name, output := range map[string]any{"claude": claudeOutput(result), "codex": codexOutput(result)} {
		raw, err := json.Marshal(output)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		json.Unmarshal(raw, &got)
		if got["decision"] != "block" || got["reason"] != shellTabMessage {
			t.Errorf("%s: %s", name, raw)
		}
		if strings.Contains(string(raw), "hookSpecificOutput") {
			t.Errorf("%s: block carries hookSpecificOutput: %s", name, raw)
		}
	}
}

func TestNonBlockOutputHasNoDecision(t *testing.T) {
	raw, _ := json.Marshal(claudeOutput(newHookResult("PostToolUse", "", []string{"note"})))
	if strings.Contains(string(raw), "decision") {
		t.Errorf("context-only output carries a decision: %s", raw)
	}
}

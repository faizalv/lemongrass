// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/agentconf"
)

func TestAgyNamedHookConfig(t *testing.T) {
	in := []byte(`{"personal":{"enabled":false,"SessionStart":[{"command":"echo user"}]},"lemongrass":{"enabled":false,"PreToolUse":[]}}`)
	out, changed, err := reconcileAgyHooks(in, testLgrass)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(out, &top); err != nil {
		t.Fatal(err)
	}
	var own map[string][]json.RawMessage
	if err := json.Unmarshal(top["lemongrass"], &own); err != nil {
		t.Fatal(err)
	}
	if len(own) != 5 {
		t.Fatalf("events = %v", own)
	}
	for _, event := range []string{"SessionStart", "PreToolUse", "PostToolUse", "PreInvocation", "Stop"} {
		if len(own[event]) != 1 {
			t.Fatalf("%s handlers = %d", event, len(own[event]))
		}
		var handler agentconf.HookHandler
		if event == "PreToolUse" || event == "PostToolUse" {
			var group agentconf.HookGroup
			if err := json.Unmarshal(own[event][0], &group); err != nil {
				t.Fatal(err)
			}
			if group.Matcher != "*" || len(group.Hooks) != 1 {
				t.Fatalf("%s = %+v", event, group)
			}
			handler = group.Hooks[0]
		} else if err := json.Unmarshal(own[event][0], &handler); err != nil {
			t.Fatal(err)
		}
		mapped := event
		if event == "PreInvocation" {
			mapped = "UserPromptSubmit"
		}
		if handler.Command != hookCmd(testLgrass, mapped, "--vendor", "agy") {
			t.Fatalf("%s = %+v", event, handler)
		}
	}
	if !strings.Contains(string(top["personal"]), "echo user") {
		t.Fatal("foreign hook lost")
	}
	if second, changed, err := reconcileAgyHooks(out, testLgrass); err != nil || changed || string(second) != string(out) {
		t.Fatalf("second pass changed=%v err=%v", changed, err)
	}
	if _, _, err := reconcileAgyHooks([]byte(`not json`), testLgrass); err == nil {
		t.Fatal("invalid config accepted")
	}
}

func TestAgyWatcherRestoresOnlyOwnedEntry(t *testing.T) {
	k, _ := fakeHome(t)
	w := startWatcher(t, k)
	hooked := func() bool {
		data, err := os.ReadFile(k.agyHooksPath())
		return err == nil && strings.Contains(string(data), "UserPromptSubmit --vendor agy")
	}
	w.waitFor(t, "agy hooks registered", hooked)
	if err := agentconf.WriteAtomic(k.agyHooksPath(), []byte(`{"personal":{"Stop":[{"command":"echo personal"}]},"lemongrass":{"enabled":false}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	w.waitFor(t, "agy hooks healed", hooked)
	data, err := os.ReadFile(k.agyHooksPath())
	if err != nil || !strings.Contains(string(data), "echo personal") {
		t.Fatalf("foreign hooks lost: %s, %v", data, err)
	}
}

func TestUninstallRemovesOnlyAgyOwnedName(t *testing.T) {
	k, _ := fakeHome(t)
	if err := agentconf.WriteAtomic(k.agyHooksPath(), []byte(`{"personal":{"Stop":[{"command":"echo personal"}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := k.reconcile(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := k.uninstall(&out, func(string) error { return nil }, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(k.agyHooksPath())
	if err != nil || strings.Contains(string(data), "lemongrass") || !strings.Contains(string(data), "echo personal") {
		t.Fatalf("uninstalled hooks = %s, %v", data, err)
	}
}

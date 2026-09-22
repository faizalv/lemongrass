package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

var claudeHookEvents = []string{"SessionStart", "SessionEnd", "PreToolUse", "PostToolUse"}

type hookHandler struct {
	Type                   string `json:"type"`
	Command                string `json:"command"`
	Timeout                int    `json:"timeout,omitempty"`
	StatusMessage          string `json:"statusMessage,omitempty"`
	AdditionalContextLimit int    `json:"additionalContextLimit,omitempty"`
}

type hookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookHandler `json:"hooks"`
}

type hookRegistration struct {
	Event   string
	Matcher string
	Handler hookHandler
}

func ownedByEvent(group json.RawMessage, event string) bool {
	var g struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if json.Unmarshal(group, &g) != nil {
		return false
	}
	suffix := " hook " + event
	for _, h := range g.Hooks {
		if strings.HasSuffix(h.Command, suffix) {
			return true
		}
	}
	return false
}

func sameJSON(a, b []byte) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return false
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func reconcileClaudeSettings(data []byte, lgrassPath string) ([]byte, bool, error) {
	registrations := make([]hookRegistration, 0, len(claudeHookEvents))
	for _, event := range claudeHookEvents {
		registrations = append(registrations, hookRegistration{
			Event:   event,
			Handler: hookHandler{Type: "command", Command: lgrassPath + " hook " + event},
		})
	}
	return reconcileHookGroups(data, registrations)
}

// Returns the input untouched and false when every Lemongrass hook group already matches.
func reconcileHookGroups(data []byte, registrations []hookRegistration) ([]byte, bool, error) {
	top := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &top); err != nil {
			return nil, false, fmt.Errorf("parsing settings: %w", err)
		}
	}

	hooks := map[string][]json.RawMessage{}
	if raw, ok := top["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return nil, false, fmt.Errorf("parsing hooks: %w", err)
		}
	}

	dirty := false
	for _, registration := range registrations {
		event := registration.Event
		desired, err := json.Marshal(hookGroup{Matcher: registration.Matcher, Hooks: []hookHandler{registration.Handler}})
		if err != nil {
			return nil, false, err
		}

		var groups []json.RawMessage
		placed := false
		for _, g := range hooks[event] {
			if !ownedByEvent(g, event) {
				groups = append(groups, g)
				continue
			}
			if placed {
				dirty = true
				continue
			}
			placed = true
			if !sameJSON(g, desired) {
				dirty = true
			}
			groups = append(groups, desired)
		}
		if !placed {
			dirty = true
			groups = append(groups, desired)
		}
		hooks[event] = groups
	}
	if !dirty {
		return data, false, nil
	}

	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		return nil, false, err
	}
	top["hooks"] = hooksJSON
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

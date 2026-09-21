package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

var hookEvents = []string{"SessionStart", "SessionEnd", "PreToolUse", "PostToolUse"}

type hookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Hooks []hookHandler `json:"hooks"`
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

// Returns the input untouched and false when every lgrass hook group already matches.
func reconcileSettings(data []byte, lgrassPath string) ([]byte, bool, error) {
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
	for _, event := range hookEvents {
		desired, err := json.Marshal(hookGroup{Hooks: []hookHandler{{Type: "command", Command: lgrassPath + " hook " + event}}})
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

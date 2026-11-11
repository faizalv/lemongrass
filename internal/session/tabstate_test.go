// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"testing"
	"time"
)

func TestOnlyAnOpenPromptHoldsANudge(t *testing.T) {
	store := openTestStore(t)

	if ready, err := store.TabReadyForNudge(tabA); err != nil || !ready {
		t.Errorf("a tab with no state = %v, %v, want ready", ready, err)
	}
	for state, want := range map[string]bool{StateIdle: true, StateWorking: true, StatePrompting: false} {
		if err := store.SetTabState(tabA, state); err != nil {
			t.Fatalf("SetTabState(%s): %v", state, err)
		}
		if ready, _ := store.TabReadyForNudge(tabA); ready != want {
			t.Errorf("state %s ready = %v, want %v", state, ready, want)
		}
	}
	store.ClearTabState(tabA)
	if ready, _ := store.TabReadyForNudge(tabA); !ready {
		t.Error("a cleared state should count as ready")
	}
}

func TestTabStatesListsThisProjectsRegisteredTabs(t *testing.T) {
	store := openTestStore(t)

	store.RegisterTab(tabA, "claude")
	store.RegisterTab(tabB, "codex")
	store.SetTabState(tabA, StateWorking)
	store.SetTabState("unregistered-tab", StatePrompting)
	if _, err := store.db.Exec(`INSERT INTO lg_tabs (project_id, tab_id, vendor, updated_at) VALUES ('other-project', 'other-tab', 'codex', ?)`, now()); err != nil {
		t.Fatalf("seeding other project: %v", err)
	}
	store.SetTabState("other-tab", StatePrompting)

	states, err := store.TabStates()
	if err != nil {
		t.Fatalf("TabStates: %v", err)
	}
	if len(states) != 1 || states[tabA] != StateWorking {
		t.Errorf("TabStates = %v, want only %s working", states, tabA)
	}
}

// Only a prompt or tool call records activity; every other state change keeps the last one, and clearing the state forgets it.
func TestOnlyActiveStateChangesRecordActivity(t *testing.T) {
	store := openTestStore(t)

	if _, ok, err := store.TabLastActive(tabA); err != nil || ok {
		t.Errorf("a tab with no state = %v, %v, want no activity", ok, err)
	}
	store.SetTabState(tabA, StateIdle)
	if _, ok, _ := store.TabLastActive(tabA); ok {
		t.Error("SetTabState recorded activity")
	}
	before := time.Now().Add(-time.Second)
	store.SetActiveTabState(tabA, StateWorking)
	first, ok, err := store.TabLastActive(tabA)
	if err != nil || !ok || first.Before(before) {
		t.Fatalf("after SetActiveTabState = %v, %v, %v, want now", first, ok, err)
	}
	store.SetTabState(tabA, StatePrompting)
	if at, ok, _ := store.TabLastActive(tabA); !ok || !at.Equal(first) {
		t.Errorf("SetTabState moved the activity to %v, want %v kept", at, first)
	}
	if state, _ := store.TabState(tabA); state != StatePrompting {
		t.Errorf("state = %s, want prompting", state)
	}
	store.ClearTabState(tabA)
	if _, ok, _ := store.TabLastActive(tabA); ok {
		t.Error("a cleared state kept its activity")
	}
}

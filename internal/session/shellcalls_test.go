// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package session

import (
	"testing"
	"time"
)

func TestShellCallStartIsReturnedOnceByItsKey(t *testing.T) {
	store := openTestStore(t)
	at := time.Date(2026, 11, 24, 10, 0, 0, 123456789, time.UTC)
	if err := store.StartShellCall("s1", "call-1", at); err != nil {
		t.Fatalf("StartShellCall: %v", err)
	}
	if err := store.StartShellCall("s1", "call-2", at.Add(time.Second)); err != nil {
		t.Fatalf("StartShellCall: %v", err)
	}
	if _, ok, _ := store.FinishShellCall("s2", "call-1"); ok {
		t.Error("another session's call was finished")
	}
	got, ok, err := store.FinishShellCall("s1", "call-1")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("FinishShellCall = %v, %v, %v; want %v", got, ok, err, at)
	}
	if _, ok, _ := store.FinishShellCall("s1", "call-1"); ok {
		t.Error("a finished call was returned twice")
	}
	if _, ok, _ := store.FinishShellCall("s1", "call-2"); !ok {
		t.Error("the second pending call was lost")
	}
}

func TestShellCallWithoutIDKeepsTheLatestStart(t *testing.T) {
	store := openTestStore(t)
	first := time.Now().Add(-time.Minute)
	store.StartShellCall("s1", "", first)
	store.StartShellCall("s1", "", first.Add(30*time.Second))
	got, ok, _ := store.FinishShellCall("s1", "")
	if !ok || !got.Equal(first.Add(30*time.Second).UTC()) {
		t.Errorf("FinishShellCall = %v, %v; want the later start", got, ok)
	}
}

func TestStaleAndDroppedShellCallsAreForgotten(t *testing.T) {
	store := openTestStore(t)
	store.StartShellCall("s1", "old", time.Now().Add(-2*shellCallKeep))
	store.StartShellCall("s1", "new", time.Now())
	if _, ok, _ := store.FinishShellCall("s1", "old"); ok {
		t.Error("a start older than the keep window survived the next start")
	}
	store.StartShellCall("s2", "pending", time.Now())
	if err := store.DropShellCalls("s2"); err != nil {
		t.Fatalf("DropShellCalls: %v", err)
	}
	if _, ok, _ := store.FinishShellCall("s2", "pending"); ok {
		t.Error("a dropped session's start survived")
	}
	if _, ok, _ := store.FinishShellCall("s1", "new"); !ok {
		t.Error("dropping one session forgot another's start")
	}
}

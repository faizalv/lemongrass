package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/session"
)

const (
	kindsLeader = "aaaaaaaa-1111-4111-8111-111111111111"
	kindsOne    = "bbbbbbbb-2222-4222-8222-222222222222"
	kindsTwo    = "cccccccc-3333-4333-8333-333333333333"
)

func storeWithKindsGroup(t *testing.T) (*session.Store, session.Group) {
	t.Helper()
	store := openHookTestStore(t)
	store.RegisterTab(kindsLeader, "claude")
	g, err := store.CreateGroup("Review", session.Member{TabID: kindsLeader, Label: "lead", Vendor: "claude"}, []session.Member{
		{TabID: kindsOne, Label: "reviewer", Vendor: "claude"},
		{TabID: kindsTwo, Label: "tester", Vendor: "claude"},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	return store, g
}

func TestHookSurfacesNotYouLinesButTheNudgeDoesNot(t *testing.T) {
	store, g := storeWithKindsGroup(t)
	store.PostMessage(kindsLeader, g.ThreadID, "for one !>>"+kindsOne+"<<!")

	if text, ok := store.ConsumePending(kindsTwo); ok {
		t.Fatalf("a nudge for an unmentioned member was composed: %q", text)
	}
	got := notificationContext(store, kindsTwo)
	if len(got) != 1 || !strings.Contains(got[0], "1 for reviewer, not you") {
		t.Fatalf("hook context = %q, want the not you line", got)
	}
	if again := notificationContext(store, kindsTwo); len(again) != 0 {
		t.Errorf("the not you line repeated: %q", again)
	}
}

func TestNudgeSettlesTheNotYouRowsOfThatTab(t *testing.T) {
	store, g := storeWithKindsGroup(t)
	store.PostMessage(kindsOne, g.ThreadID, "to lead !>>"+kindsLeader+"<<!")
	store.PostMessage(kindsLeader, g.ThreadID, "to all")

	text, ok := store.ConsumePending(kindsTwo)
	if !ok || strings.Contains(text, "not you") || !strings.Contains(text, "1 new from lead") {
		t.Fatalf("nudge = %q, %v, want the plain message only", text, ok)
	}
	if got := notificationContext(store, kindsTwo); len(got) != 0 {
		t.Errorf("the not you row surfaced after the nudge: %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = saved
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintThreadShowsOnlyUnreadAfterTheFirstRead(t *testing.T) {
	store, g := storeWithKindsGroup(t)
	tabID = kindsOne
	t.Cleanup(func() { tabID = "" })
	args := threadArgs{limit: defaultThreadReadLimit}

	store.PostMessage(kindsLeader, g.ThreadID, "first news")
	first := captureStdout(t, func() { printThread(store, g.ThreadID, args, nil) })
	if !strings.Contains(first, "first news") || strings.Contains(first, "unread only") {
		t.Fatalf("first read = %q, want the latest page", first)
	}

	store.PostMessage(kindsLeader, g.ThreadID, "second news")
	second := captureStdout(t, func() { printThread(store, g.ThreadID, args, nil) })
	if !strings.Contains(second, "second news") || strings.Contains(second, "first news") || !strings.Contains(second, "unread only") {
		t.Errorf("second read = %q, want only the new message", second)
	}

	third := captureStdout(t, func() { printThread(store, g.ThreadID, args, nil) })
	if !strings.Contains(third, "nothing new") {
		t.Errorf("third read = %q, want the nothing new line", third)
	}

	all := captureStdout(t, func() { printThread(store, g.ThreadID, threadArgs{limit: 10, all: true}, nil) })
	if !strings.Contains(all, "first news") || !strings.Contains(all, "second news") {
		t.Errorf("--all read = %q, want the latest page", all)
	}
}

func TestPrintThreadWithoutATabKeepsTheLatestPage(t *testing.T) {
	store, g := storeWithKindsGroup(t)
	tabID = ""
	store.PostMessage(kindsLeader, g.ThreadID, "one")
	for i := 0; i < 2; i++ {
		out := captureStdout(t, func() { printThread(store, g.ThreadID, threadArgs{limit: 10}, nil) })
		if !strings.Contains(out, "one") || strings.Contains(out, "unread only") {
			t.Errorf("read %d = %q, want the full latest page every time", i, out)
		}
	}
}

func TestPrintThreadHeaderIsFullOnlyOnTheFirstRead(t *testing.T) {
	store, g := storeWithKindsGroup(t)
	tabID = kindsTwo
	t.Cleanup(func() { tabID = "" })
	var firsts []bool
	header := func(first bool) string {
		firsts = append(firsts, first)
		return "header"
	}
	captureStdout(t, func() { printThread(store, g.ThreadID, threadArgs{limit: 10}, header) })
	store.PostMessage(kindsLeader, g.ThreadID, "x")
	captureStdout(t, func() { printThread(store, g.ThreadID, threadArgs{limit: 10}, header) })
	captureStdout(t, func() { printThread(store, g.ThreadID, threadArgs{limit: 10, all: true}, header) })
	if len(firsts) != 3 || !firsts[0] || firsts[1] || !firsts[2] {
		t.Errorf("first flags = %v, want true, false, true", firsts)
	}
}

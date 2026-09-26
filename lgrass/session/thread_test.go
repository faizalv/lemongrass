package session

import (
	"strings"
	"testing"
)

const (
	tabA = "aaaaaaaa-1111-4111-8111-111111111111"
	tabB = "bbbbbbbb-2222-4222-8222-222222222222"
	tabC = "bbbbbbbb-3333-4333-8333-333333333333"
)

func TestCreateThreadStoresTitleAndFirstMessage(t *testing.T) {
	store := openTestStore(t)

	id, err := store.CreateThread(tabA, "  Schema review  ", "first message")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	thread, err := store.ThreadByID(id)
	if err != nil {
		t.Fatalf("ThreadByID: %v", err)
	}
	if thread.Title != "Schema review" || thread.CreatedBy != tabA || thread.MessageCount != 1 || thread.GroupID != 0 {
		t.Errorf("thread = %+v, want trimmed title, creator tabA, one message, no group", thread)
	}
	msgs, more, err := store.ReadThread(id, 0, 10)
	if err != nil || more || len(msgs) != 1 || msgs[0].Body != "first message" {
		t.Errorf("ReadThread = %+v, %v, %v, want the one first message", msgs, more, err)
	}
}

func TestThreadRejectsEmptyAndOversizedInput(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.CreateThread(tabA, "", "body"); err == nil {
		t.Error("empty title accepted")
	}
	if _, err := store.CreateThread(tabA, strings.Repeat("t", MaxTitleRunes+1), "body"); err == nil {
		t.Error("oversized title accepted")
	}
	if _, err := store.CreateThread(tabA, "title", "   "); err == nil {
		t.Error("blank message accepted")
	}
	_, err := store.CreateThread(tabA, "title", strings.Repeat("é", MaxMessageRunes+1))
	if err == nil || !strings.Contains(err.Error(), "scratchpad note") {
		t.Errorf("oversized message error = %v, want a pointer to a scratchpad note", err)
	}
	if _, err := store.CreateThread(tabA, "title", strings.Repeat("é", MaxMessageRunes)); err != nil {
		t.Errorf("message at exactly the cap in runes rejected: %v", err)
	}
}

func TestPostMessageToMissingOrOtherProjectThread(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.PostMessage(tabA, 999, "hello"); err != ErrNoSuchThread {
		t.Errorf("post to missing thread error = %v, want ErrNoSuchThread", err)
	}

	id, _ := store.CreateThread(tabA, "title", "hello")
	other := &Store{db: store.db, projectID: "another-project"}
	if _, err := other.PostMessage(tabB, id, "hi"); err != ErrNoSuchThread {
		t.Errorf("post from another project error = %v, want ErrNoSuchThread", err)
	}
	if _, err := other.ThreadByID(id); err != ErrNoSuchThread {
		t.Errorf("read from another project error = %v, want ErrNoSuchThread", err)
	}
}

func TestReadThreadPaginatesNewestFirst(t *testing.T) {
	store := openTestStore(t)

	id, _ := store.CreateThread(tabA, "title", "m1")
	for _, body := range []string{"m2", "m3", "m4", "m5"} {
		if _, err := store.PostMessage(tabB, id, body); err != nil {
			t.Fatalf("PostMessage: %v", err)
		}
	}

	page1, more, err := store.ReadThread(id, 0, 2)
	if err != nil || !more || len(page1) != 2 || page1[0].Body != "m5" || page1[1].Body != "m4" {
		t.Fatalf("page1 = %+v, more %v, err %v, want m5 m4 with more", page1, more, err)
	}
	page2, more, err := store.ReadThread(id, page1[1].ID, 2)
	if err != nil || !more || len(page2) != 2 || page2[0].Body != "m3" || page2[1].Body != "m2" {
		t.Fatalf("page2 = %+v, more %v, err %v, want m3 m2 with more", page2, more, err)
	}
	page3, more, err := store.ReadThread(id, page2[1].ID, 2)
	if err != nil || more || len(page3) != 1 || page3[0].Body != "m1" {
		t.Fatalf("page3 = %+v, more %v, err %v, want m1 and no more", page3, more, err)
	}
}

func TestListThreadsMostRecentlyActiveFirst(t *testing.T) {
	store := openTestStore(t)

	first, _ := store.CreateThread(tabA, "first", "a")
	second, _ := store.CreateThread(tabA, "second", "b")
	store.PostMessage(tabB, first, "reply")

	threads, err := store.ListThreads(10)
	if err != nil || len(threads) != 2 {
		t.Fatalf("ListThreads = %+v, %v, want 2 threads", threads, err)
	}
	if threads[0].ID != first || threads[0].MessageCount != 2 || threads[1].ID != second {
		t.Errorf("order = %d then %d, want the replied-to thread %d first", threads[0].ID, threads[1].ID, first)
	}
	if limited, _ := store.ListThreads(1); len(limited) != 1 {
		t.Errorf("limit 1 returned %d threads", len(limited))
	}
}

func TestParseMentions(t *testing.T) {
	got := ParseMentions("ping !>>aaaaaaaa<<! and !>> bbbbbbbb <<! and again !>>aaaaaaaa<<! ok !>><<!")
	want := []string{"aaaaaaaa", "bbbbbbbb", ""}
	if len(got) != len(want) {
		t.Fatalf("ParseMentions = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
	if got := ParseMentions("no markers here !> nope <!"); len(got) != 0 {
		t.Errorf("ParseMentions on plain text = %q, want none", got)
	}
}

func TestMentionsResolveByUniquePrefix(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabA, "claude")
	store.RegisterTab(tabB, "codex")
	store.RegisterTab(tabC, "codex")

	id, err := store.CreateThread(tabA, "title", "hey !>>bbbbbbbb-2<<! and !>>"+tabA+"<<!")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	msgs, _, _ := store.ReadThread(id, 0, 10)
	if len(msgs[0].Mentions) != 2 {
		t.Fatalf("mentions = %q, want tabA and tabB", msgs[0].Mentions)
	}
	if msgs[0].Mentions[0] != tabA || msgs[0].Mentions[1] != tabB {
		t.Errorf("mentions = %q, want [%s %s]", msgs[0].Mentions, tabA, tabB)
	}
}

func TestMentionRejectsShortUnknownAmbiguousAndMalformed(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabB, "codex")
	store.RegisterTab(tabC, "codex")
	id, _ := store.CreateThread(tabB, "title", "hello")

	cases := map[string]string{
		"short":     "!>>bbbb<<!",
		"unknown":   "!>>cccccccc<<!",
		"ambiguous": "!>>bbbbbbbb<<!",
		"malformed": "!>>not-a-hex-id<<!",
		"empty":     "!>><<!",
	}
	for name, body := range cases {
		if _, err := store.PostMessage(tabB, id, body); err == nil {
			t.Errorf("%s mention accepted", name)
		}
	}
	msgs, _, _ := store.ReadThread(id, 0, 10)
	if len(msgs) != 1 {
		t.Errorf("a rejected post was stored: %d messages", len(msgs))
	}
}

func TestMentionResolvesAGroupMemberWithoutATabRecord(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.db.Exec(`INSERT INTO lg_groups (id, project_id, pilot_tab_id, created_at) VALUES (1, ?, ?, ?)`, testProjectID, tabA, now()); err != nil {
		t.Fatalf("seeding group: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO lg_group_members (group_id, tab_id, role, label, vendor) VALUES (1, ?, 'copilot', 'reviewer', 'claude')`, tabB); err != nil {
		t.Fatalf("seeding member: %v", err)
	}

	got, err := store.ResolveMentions([]string{"bbbbbbbb"})
	if err != nil || len(got) != 1 || got[0] != tabB {
		t.Errorf("ResolveMentions = %q, %v, want the closed member's tab id", got, err)
	}
}

func TestFormatThreadReadShowsFramingMentionsAndNextPage(t *testing.T) {
	thread := Thread{ID: 7, Title: "Review", CreatedBy: tabA, CreatedAt: "2026-09-26T10:00:00Z", MessageCount: 3}
	msgs := []Message{
		{ID: 9, TabID: tabB, Body: "second", CreatedAt: "2026-09-26T10:05:00Z", Mentions: []string{tabA}},
		{ID: 8, TabID: tabA, Body: "first", CreatedAt: "2026-09-26T10:01:00Z"},
	}
	out := FormatThreadRead(thread, msgs, true)
	for _, want := range []string{"thread 7 [Review]", "not your user", "#9", "bbbbbbbb", "(mentions aaaaaaaa)", "lgrass thread read 7 --before 8"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(FormatThreadRead(thread, msgs, false), "older messages") {
		t.Error("next-page hint shown with no older messages")
	}
}

package threadsvc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/session"
)

const (
	tabClaude = "aaaaaaaa-1111-4111-8111-111111111111"
	tabCodex  = "bbbbbbbb-2222-4222-8222-222222222222"
	tabAuthor = "cccccccc-3333-4333-8333-333333333333"
)

func openStore(t *testing.T) *session.Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := session.Open(session.DBPath(), "proj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	store.RegisterTab(tabClaude, "claude")
	store.RegisterTab(tabCodex, "codex")
	store.RegisterTab(tabAuthor, "codex")
	return store
}

func liveClaudeSession(store *session.Store, socket string) {
	store.Start("session-claude", socket, "tok")
	store.RecordTabSession(tabClaude, "session-claude")
}

type recorder struct {
	mu    sync.Mutex
	texts []string
	fail  bool
}

func (r *recorder) push(_ session.MessagingTarget, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("push failed")
	}
	r.texts = append(r.texts, text)
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.texts)
}

func TestBackoffDoublesAndFitsFiveAttemptsInTenMinutes(t *testing.T) {
	var total time.Duration
	for attempts := 1; attempts < MaxAttempts; attempts++ {
		total += Backoff(attempts)
	}
	if Backoff(0) != 0 || Backoff(1) != 30*time.Second || Backoff(2) != time.Minute {
		t.Errorf("backoff = %v %v %v, want 0 30s 1m", Backoff(0), Backoff(1), Backoff(2))
	}
	if total > 10*time.Minute {
		t.Errorf("waits before the fifth attempt total %v, want within 10m", total)
	}
}

func TestDeliverTabPushesOneCoalescedNudgeAndMarksSent(t *testing.T) {
	store := openStore(t)
	liveClaudeSession(store, "/tmp/x.sock")
	id, _ := store.CreateThread(tabAuthor, "Review", "a !>>"+tabClaude+"<<!")
	store.PostMessage(tabAuthor, id, "b !>>"+tabClaude+"<<!")

	rec := &recorder{}
	d := &Deliverer{Store: store, Push: rec.push}
	sent, err := d.DeliverTab(tabClaude)
	if err != nil || !sent {
		t.Fatalf("DeliverTab = %v, %v, want a push", sent, err)
	}
	if rec.count() != 1 || !strings.Contains(rec.texts[0], "2 new messages in thread") || strings.Contains(rec.texts[0], "a !>>") {
		t.Errorf("pushed %q, want one coalesced nudge without content", rec.texts)
	}
	if pending, _ := store.PendingForTab(tabClaude); len(pending) != 0 {
		t.Errorf("rows still pending after a push: %+v", pending)
	}
	if sent, _ := d.DeliverTab(tabClaude); sent || rec.count() != 1 {
		t.Error("a second delivery pushed again")
	}
}

func TestDeliverTabFailureAndNoSessionCountAttemptsAndStayPending(t *testing.T) {
	store := openStore(t)
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabClaude+"<<!")
	rec := &recorder{}
	d := &Deliverer{Store: store, Push: rec.push}

	if sent, _ := d.DeliverTab(tabClaude); sent {
		t.Error("pushed to a tab with no live session")
	}
	liveClaudeSession(store, "/tmp/x.sock")
	rec.fail = true
	if sent, _ := d.DeliverTab(tabClaude); sent {
		t.Error("reported a push that failed")
	}
	pending, _ := store.PendingForTab(tabClaude)
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want the row kept", pending)
	}
	if due, _ := store.TabsDueForRetry(MaxAttempts, Backoff, time.Now().Add(time.Hour)); len(due) != 1 {
		t.Errorf("row with 2 attempts not due after the backoff: %v", due)
	}
}

func TestDeliverTabLeavesNonClaudeTabsPending(t *testing.T) {
	store := openStore(t)
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabCodex+"<<!")
	rec := &recorder{}
	d := &Deliverer{Store: store, Push: rec.push}

	if sent, err := d.DeliverTab(tabCodex); sent || err != nil || rec.count() != 0 {
		t.Errorf("codex tab: sent %v, err %v, pushes %d, want none", sent, err, rec.count())
	}
	if pending, _ := store.PendingForTab(tabCodex); len(pending) != 1 {
		t.Error("codex row not left pending")
	}
}

func startService(t *testing.T, store *session.Store) (*Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "threads.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	svc := NewService(store)
	go svc.Serve(l)
	return svc, path
}

func TestWakeDeliversToClaudeOverTheSocket(t *testing.T) {
	store := openStore(t)
	claudeLn, err := net.Listen("unix", filepath.Join(t.TempDir(), "claude.sock"))
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer claudeLn.Close()
	got := make(chan string, 1)
	go func() {
		conn, err := claudeLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var lines []string
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		if len(lines) == 2 {
			var m struct {
				Message struct{ Content string } `json:"message"`
			}
			json.Unmarshal([]byte(lines[1]), &m)
			got <- m.Message.Content
		}
	}()
	liveClaudeSession(store, claudeLn.Addr().String())
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabClaude+"<<!")
	_, path := startService(t, store)

	if err := Wake(path); err != nil {
		t.Fatalf("Wake: %v", err)
	}
	select {
	case text := <-got:
		if !strings.Contains(text, "1 new message in thread 1 [Review]") {
			t.Errorf("claude received %q", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("claude socket received nothing")
	}
}

func TestWaitReturnsImmediatelyWhenRowsAlreadyPending(t *testing.T) {
	store := openStore(t)
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabCodex+"<<!")
	_, path := startService(t, store)

	start := time.Now()
	woke, err := Wait(path, tabCodex, 10)
	if err != nil || !woke || time.Since(start) > 2*time.Second {
		t.Errorf("Wait = %v, %v after %v, want an immediate wake", woke, err, time.Since(start))
	}
}

func TestWaitWakesWhenAMessageArrivesLater(t *testing.T) {
	store := openStore(t)
	_, path := startService(t, store)

	type result struct {
		woke bool
		err  error
	}
	done := make(chan result, 1)
	go func() {
		woke, err := Wait(path, tabCodex, 10)
		done <- result{woke, err}
	}()
	time.Sleep(200 * time.Millisecond)
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabCodex+"<<!")
	if err := Wake(path); err != nil {
		t.Fatalf("Wake: %v", err)
	}
	select {
	case r := <-done:
		if r.err != nil || !r.woke {
			t.Errorf("Wait = %v, %v, want a wake", r.woke, r.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked listener was not woken")
	}
}

func TestWaitTimesOutWithoutAWake(t *testing.T) {
	store := openStore(t)
	_, path := startService(t, store)

	woke, err := Wait(path, tabCodex, 1)
	if err != nil || woke {
		t.Errorf("Wait = %v, %v, want a quiet timeout", woke, err)
	}
}

func TestClientsReportAnUnreachableService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.sock")
	if err := Wake(path); err == nil {
		t.Error("Wake reported success with no service")
	}
	if _, err := Wait(path, tabCodex, 1); err == nil {
		t.Error("Wait reported success with no service")
	}
}

func TestRunScansAtStartAndRetriesDueRows(t *testing.T) {
	store := openStore(t)
	liveClaudeSession(store, "/tmp/x.sock")
	store.CreateThread(tabAuthor, "Review", "a !>>"+tabClaude+"<<!")
	rec := &recorder{fail: true}
	svc := NewService(store)
	svc.deliver.Push = rec.push
	svc.now = func() time.Time { return time.Now().Add(time.Hour) }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx, 60*time.Millisecond, time.Hour); close(done) }()

	time.Sleep(15 * time.Millisecond)
	rec.mu.Lock()
	rec.fail = false
	rec.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for rec.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	if rec.count() != 1 {
		t.Errorf("pushes after retry = %d, want 1", rec.count())
	}
	if pending, _ := store.PendingForTab(tabClaude); len(pending) != 0 {
		t.Error("row still pending after a successful retry")
	}
}

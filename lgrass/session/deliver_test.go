package session

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// listenTestSocket starts a real unix socket listener at a fresh temp
// path and returns it plus that path, for a test to dial with Deliver.
func listenTestSocket(t *testing.T) (net.Listener, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln, path
}

func TestDeliverSendsAuthThenMessage(t *testing.T) {
	ln, path := listenTestSocket(t)

	received := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var lines []string
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		received <- lines
	}()

	err := Deliver(MessagingTarget{SessionID: "session-b", Socket: path, Token: "tok-123"}, "hello from session-a")
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	select {
	case lines := <-received:
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2 (auth + message): %v", len(lines), lines)
		}
		var auth authLine
		if err := json.Unmarshal([]byte(lines[0]), &auth); err != nil {
			t.Fatalf("unmarshaling auth line: %v", err)
		}
		if auth.Type != "auth" || auth.Token != "tok-123" {
			t.Errorf("auth line = %+v, want type=auth token=tok-123", auth)
		}
		var msg userMessage
		if err := json.Unmarshal([]byte(lines[1]), &msg); err != nil {
			t.Fatalf("unmarshaling message line: %v", err)
		}
		if msg.Type != "user" || msg.Message.Role != "user" || msg.Message.Content != "hello from session-a" {
			t.Errorf("message line = %+v, want type=user role=user content=%q", msg, "hello from session-a")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delivered lines")
	}
}

func TestDeliverOmitsAuthLineWhenTokenEmpty(t *testing.T) {
	ln, path := listenTestSocket(t)

	received := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var lines []string
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		received <- lines
	}()

	if err := Deliver(MessagingTarget{SessionID: "session-b", Socket: path}, "no token here"); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	select {
	case lines := <-received:
		if len(lines) != 1 {
			t.Fatalf("got %d lines, want 1 (message only, no auth line)", len(lines))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delivered lines")
	}
}

func TestDeliverErrorsOnMissingSocket(t *testing.T) {
	err := Deliver(MessagingTarget{SessionID: "gone", Socket: "/nonexistent/path/does-not-exist.sock"}, "x")
	if err == nil {
		t.Fatal("Deliver against a nonexistent socket path should return an error")
	}
}

func TestDeliverAllContinuesPastFailuresAndCountsSuccesses(t *testing.T) {
	ln, path := listenTestSocket(t)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	targets := []MessagingTarget{
		{SessionID: "good", Socket: path},
		{SessionID: "bad", Socket: "/nonexistent/does-not-exist.sock"},
	}
	delivered := DeliverAll(targets, "hi")
	if delivered != 1 {
		t.Fatalf("DeliverAll delivered = %d, want 1 (one good, one bad target)", delivered)
	}
}

package agent

import (
	"bytes"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

func startTestAgent(t *testing.T, vaultClient *vault.Client) *Client {
	t.Helper()
	svc := NewService(vaultClient)
	sockPath := filepath.Join(t.TempDir(), "agent.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	limiter := vault.NewFailureLimiter(1000, time.Minute, time.Hour)
	go Serve(svc, l, limiter)
	t.Cleanup(func() { l.Close() })
	return &Client{SocketPath: sockPath}
}

func TestIPCFullLifecycle(t *testing.T) {
	vaultClient := startTestVault(t)
	want := []byte("postgres://kencana-backend-creds")
	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", want); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	agentClient := startTestAgent(t, vaultClient)
	shortID, err := agentClient.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}
	if shortID == "" {
		t.Fatal("RegisterChannel returned an empty short id")
	}

	got, err := agentClient.Query(shortID, "employees", "select")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Query returned %q, want %q", got, want)
	}

	if err := agentClient.Forget(shortID); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if _, err := agentClient.Query(shortID, "employees", "select"); err == nil {
		t.Error("Query after Forget returned nil error")
	}
}

func TestIPCQueryLocksOutAfterRepeatedWrongShortID(t *testing.T) {
	vaultClient := startTestVault(t)
	svc := NewService(vaultClient)
	sockPath := filepath.Join(t.TempDir(), "agent.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	limiter := vault.NewFailureLimiter(3, time.Minute, time.Hour)
	go Serve(svc, l, limiter)
	agentClient := &Client{SocketPath: sockPath}

	for i := 0; i < 3; i++ {
		if _, err := agentClient.Query("BOGUS1", "employees", "select"); err == nil {
			t.Fatalf("Query with a bogus short id (attempt %d) returned nil error", i)
		}
	}

	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	shortID, err := svc.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	// The limiter should now be locked out even for a real, correctly-registered short id.
	if _, err := agentClient.Query(shortID, "employees", "select"); err == nil {
		t.Error("Query with a valid id after lockout returned nil error")
	}
}

func TestIPCRegisterChannelIsNotQueryGated(t *testing.T) {
	vaultClient := startTestVault(t)
	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	agentClient := startTestAgent(t, vaultClient)

	// Many RegisterChannel calls in a row should never trip the query limiter.
	for i := 0; i < 20; i++ {
		if _, err := agentClient.RegisterChannel(c.ID); err != nil {
			t.Fatalf("RegisterChannel attempt %d: %v", i, err)
		}
	}
}

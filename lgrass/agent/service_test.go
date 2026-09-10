package agent

import (
	"bytes"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

const testRootSecret = "correct horse battery staple"

func fullScope() vault.Scope {
	return vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
}

// startTestVault brings up a real vault daemon over its own unix socket.
func startTestVault(t *testing.T) *vault.Client {
	t.Helper()
	svc, err := vault.NewService(t.TempDir())
	if err != nil {
		t.Fatalf("vault.NewService: %v", err)
	}
	sockPath := filepath.Join(t.TempDir(), "vault.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	limiter := vault.NewFailureLimiter(1000, time.Minute, time.Hour)
	go vault.Serve(svc, l, limiter)
	t.Cleanup(func() { l.Close() })
	return &vault.Client{SocketPath: sockPath}
}

func TestServiceRegisterThenQuery(t *testing.T) {
	vaultClient := startTestVault(t)
	want := []byte("postgres://kencana-backend-creds")
	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", want); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	svc := NewService(vaultClient)
	shortID, err := svc.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}
	if len(shortID) != shortIDLength {
		t.Errorf("RegisterChannel returned id of length %d, want %d", len(shortID), shortIDLength)
	}

	got, err := svc.Query(shortID, "employees", "select")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Query returned %q, want %q", got, want)
	}
}

func TestServiceRegisterChannelRejectsUnknownRealID(t *testing.T) {
	vaultClient := startTestVault(t)
	svc := NewService(vaultClient)
	if _, err := svc.RegisterChannel("nonexistent-real-id"); err == nil {
		t.Error("RegisterChannel with an unknown real channel id returned nil error")
	}
}

func TestServiceQueryUnknownShortIDReturnsErrNoSuchChannel(t *testing.T) {
	svc := NewService(startTestVault(t))
	if _, err := svc.Query("BOGUS1", "employees", "select"); !errors.Is(err, ErrNoSuchChannel) {
		t.Errorf("Query with an unregistered short id: got %v, want ErrNoSuchChannel", err)
	}
}

func TestServiceForgetThenQueryReturnsErrNoSuchChannel(t *testing.T) {
	vaultClient := startTestVault(t)
	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	svc := NewService(vaultClient)
	shortID, err := svc.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	svc.Forget(shortID)
	if _, err := svc.Query(shortID, "employees", "select"); !errors.Is(err, ErrNoSuchChannel) {
		t.Errorf("Query after Forget: got %v, want ErrNoSuchChannel", err)
	}
}

func TestServiceForgetUnknownShortIDIsNoop(t *testing.T) {
	svc := NewService(startTestVault(t))
	svc.Forget("BOGUS1") // must not panic on a mapping that was never there
}

func TestServiceQueryOnOutOfScopeTableFails(t *testing.T) {
	vaultClient := startTestVault(t)
	if err := vaultClient.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := vaultClient.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	svc := NewService(vaultClient)
	shortID, err := svc.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	if _, err := svc.Query(shortID, "salaries", "select"); err == nil {
		t.Error("Query on an out-of-scope table returned nil error")
	}
}

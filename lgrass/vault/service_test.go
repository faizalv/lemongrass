package vault

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

const testRootSecret = "correct horse battery staple"

func openTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func fullScope() Scope {
	return Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
}

func TestServiceCreateChannelThenQuery(t *testing.T) {
	svc := openTestService(t)
	want := []byte("postgres://kencana-backend-creds")
	if err := svc.PutCredential(testRootSecret, "kencana-backend", want); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	got, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Query returned %q, want %q", got, want)
	}
}

func TestServiceQueryDeniesOutOfScope(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := svc.Query(c.ID, Query{Table: "salaries", Operation: "select"}); err == nil {
		t.Error("Query on an out-of-scope table returned nil error")
	}
}

func TestServiceQueryFailsAfterExpiry(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), -1*time.Second)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"}); err == nil {
		t.Error("Query on an already-expired channel returned nil error")
	}
}

func TestServiceQueryFailsAfterRevoke(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if err := svc.Revoke(c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Query after Revoke: got %v, want ErrNotFound", err)
	}
}

func TestServiceActivateReusesWrappedCopyWithoutRootCredential(t *testing.T) {
	svc := openTestService(t)
	want := []byte("postgres://kencana-backend-creds")
	if err := svc.PutCredential(testRootSecret, "kencana-backend", want); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), -1*time.Second)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"}); err == nil {
		t.Fatal("expected the freshly-created channel to already be expired")
	}

	// Deleting the base credential proves Activate never touches it again -- only the already-wrapped per-channel copy.
	if err := svc.creds.Delete("kencana-backend"); err != nil {
		t.Fatalf("deleting base credential: %v", err)
	}

	if _, err := svc.Activate(testRootSecret, c.ID, 5*time.Minute); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	got, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"})
	if err != nil {
		t.Fatalf("Query after Activate: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Query after Activate returned %q, want %q", got, want)
	}
}

func TestServiceQueryFailsWhenNeverActivated(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// Simulate a vault restart: metadata and the wrapped copy persist on disk, but the in-memory key cache is gone.
	svc.mu.Lock()
	delete(svc.activeKeys, c.ID)
	svc.mu.Unlock()

	if _, err := svc.Query(c.ID, Query{Table: "employees", Operation: "select"}); err == nil {
		t.Error("Query on a channel with no cached key returned nil error")
	}
}

func TestServiceCreateChannelWrongRootSecretFails(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	if _, err := svc.CreateChannel("wrong passphrase", "kencana-backend", fullScope(), 5*time.Minute); err == nil {
		t.Error("CreateChannel with the wrong root secret returned nil error")
	}
}

func TestServiceChannelScopeReadableWithoutActivation(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	svc.mu.Lock()
	delete(svc.activeKeys, c.ID)
	svc.mu.Unlock()

	got, err := svc.ChannelScope(c.ID)
	if err != nil {
		t.Fatalf("ChannelScope on an inactive channel: %v", err)
	}
	if got.DBName != "kencana-backend" {
		t.Errorf("ChannelScope.DBName = %q, want kencana-backend", got.DBName)
	}
}

func TestServiceListChannels(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	empty, err := svc.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels on an empty vault: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListChannels on an empty vault = %v, want none", empty)
	}

	a, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel a: %v", err)
	}
	b, err := svc.CreateChannel(testRootSecret, "kencana-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel b: %v", err)
	}

	channels, err := svc.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("len(ListChannels()) = %d, want 2", len(channels))
	}
	ids := map[ChannelID]bool{channels[0].ID: true, channels[1].ID: true}
	if !ids[a.ID] || !ids[b.ID] {
		t.Errorf("ListChannels() = %v, want to include %s and %s", channels, a.ID, b.ID)
	}

	if err := svc.Revoke(a.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	channels, err = svc.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels after revoke: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != b.ID {
		t.Fatalf("ListChannels after revoking a = %v, want only %s", channels, b.ID)
	}
}

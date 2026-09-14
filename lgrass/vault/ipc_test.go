package vault

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func startTestServer(t *testing.T) *Client {
	t.Helper()
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	sockPath := filepath.Join(t.TempDir(), "vault.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	limiter := NewFailureLimiter(1000, time.Minute, time.Hour)
	go Serve(svc, l, limiter)
	t.Cleanup(func() { l.Close() })
	return &Client{SocketPath: sockPath}
}

func TestIPCFullChannelLifecycle(t *testing.T) {
	client := startTestServer(t)

	if err := client.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	c, err := client.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if c.ID == "" {
		t.Fatal("CreateChannel returned an empty channel id")
	}

	scope, err := client.ChannelScope(c.ID)
	if err != nil {
		t.Fatalf("ChannelScope: %v", err)
	}
	if scope.DBName != "app-backend" {
		t.Errorf("ChannelScope.DBName = %q, want app-backend", scope.DBName)
	}

	// Reaches execution and fails only because nothing's listening -- proof the query travelled the whole IPC round trip.
	_, err = client.Query(c.ID, []string{"employees"}, "SELECT id FROM employees")
	if err == nil || !strings.Contains(err.Error(), "executing query") {
		t.Errorf("Query = %v, want it to fail at execution", err)
	}

	if _, err := client.Query(c.ID, []string{"salaries"}, "SELECT id FROM salaries"); err == nil {
		t.Error("Query on an out-of-scope table returned nil error")
	}

	if _, err := client.Activate(testRootSecret, c.ID, 10*time.Minute); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	channels, err := client.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != c.ID {
		t.Fatalf("ListChannels() = %v, want only %s", channels, c.ID)
	}

	if err := client.Revoke(c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := client.Query(c.ID, []string{"employees"}, "SELECT id FROM employees"); err == nil {
		t.Error("Query after Revoke returned nil error")
	}

	channels, err = client.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels after revoke: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("ListChannels after revoke = %v, want none", channels)
	}
}

func TestIPCAdminOpsLockOutAfterRepeatedWrongSecret(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	sockPath := filepath.Join(t.TempDir(), "vault.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	limiter := NewFailureLimiter(3, time.Minute, time.Hour)
	go Serve(svc, l, limiter)
	client := &Client{SocketPath: sockPath}

	for i := 0; i < 3; i++ {
		if _, err := client.CreateChannel("wrong passphrase", "test-channel", "app-backend", fullScope(), time.Minute); err == nil {
			t.Fatalf("CreateChannel with a wrong passphrase (attempt %d) returned nil error", i)
		}
	}

	// The limiter should now be locked out even for a correct secret.
	if _, err := client.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), time.Minute); err == nil {
		t.Error("CreateChannel with the correct secret after lockout returned nil error")
	}
}

func TestIPCListTablesRoundTrips(t *testing.T) {
	client := startTestServer(t)
	if err := client.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	// Reaches the listing query and fails only because nothing's listening -- proof it
	// travelled the whole IPC round trip, not a wrong-passphrase or dispatch failure.
	_, err := client.ListTables(testRootSecret, "app-backend")
	if err == nil || !strings.Contains(err.Error(), "listing tables") {
		t.Errorf("ListTables = %v, want it to fail at the listing query", err)
	}

	if _, err := client.ListTables("wrong passphrase", "app-backend"); err == nil {
		t.Error("ListTables with the wrong root secret returned nil error")
	}
}

func TestIPCQueryIsNotAdminGated(t *testing.T) {
	client := startTestServer(t)
	if err := client.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := client.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// Many Query calls in a row should never trip the admin limiter -- it only gates root-secret-bearing ops.
	for i := 0; i < 20; i++ {
		_, err := client.Query(c.ID, []string{"employees"}, "SELECT id FROM employees")
		if err == nil || !strings.Contains(err.Error(), "executing query") {
			t.Fatalf("Query attempt %d = %v, want it to fail at execution, not be admin-gated", i, err)
		}
	}
}

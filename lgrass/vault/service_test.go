package vault

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testRootSecret = "correct horse battery staple"

// unreachableConnString points at a port nothing listens on, on loopback -- connecting to it
// fails fast with "connection refused" rather than hanging on a timeout, so tests can drive
// Query all the way to the execution step without a real database or any real network access.
const unreachableConnString = "mysql://root:secret@127.0.0.1:1/appdb"

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

// wantsExecution asserts err is the "reached the database" failure runQuery reports for an
// unreachable connection -- proof Query got all the way through decrypt/classify/scope before
// failing, as opposed to failing at one of those earlier steps.
func wantsExecution(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Query against an unreachable database returned nil error")
	}
	if !strings.Contains(err.Error(), "vault: executing query") {
		t.Errorf("Query error = %q, want it to fail at execution, not earlier", err.Error())
	}
}

func TestServiceQueryReachesExecutionAfterScopeChecksPass(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	_, err = svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees")
	wantsExecution(t, err)
}

func TestServiceQueryDeniesOutOfScopeTable(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	_, err = svc.Query(c.ID, []string{"salaries"}, "SELECT id FROM salaries")
	var violation *ErrScopeViolation
	if !errors.As(err, &violation) {
		t.Errorf("Query on an out-of-scope table: got %v, want ErrScopeViolation", err)
	}
}

func TestServiceQueryDeniesDeclaredTableMismatch(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	scope := Scope{Tables: []string{"employees", "salaries"}, Operations: []string{"select"}}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", scope, 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// Scope grants both tables, so a declared list missing "salaries" exercises the declared-tables check alone, not a scope denial.
	_, err = svc.Query(c.ID, []string{"employees"}, "SELECT e.id FROM employees e JOIN salaries s ON s.employee_id = e.id")
	var mismatch *ErrTableMismatch
	if !errors.As(err, &mismatch) {
		t.Errorf("Query with a mismatched declared table list: got %v, want ErrTableMismatch", err)
	}
}

func TestServiceQueryDeniesIntrospectWithoutShowGranted(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	_, err = svc.Query(c.ID, []string{"*"}, "SHOW TABLES")
	var violation *ErrScopeViolation
	if !errors.As(err, &violation) {
		t.Errorf("Query for SHOW without show granted: got %v, want ErrScopeViolation", err)
	}
}

func TestServiceQueryRejectsWriteStatement(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := svc.Query(c.ID, []string{"employees"}, "DELETE FROM employees"); err == nil {
		t.Error("Query with a write statement returned nil error")
	}
}

func TestServiceQueryFailsAfterExpiry(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), -1*time.Second)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees"); err == nil {
		t.Error("Query on an already-expired channel returned nil error")
	}
}

func TestServiceQueryFailsAfterRevoke(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if err := svc.Revoke(c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Query after Revoke: got %v, want ErrNotFound", err)
	}
}

func TestServiceActivateReusesWrappedCopyWithoutRootCredential(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), -1*time.Second)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees"); err == nil {
		t.Fatal("expected the freshly-created channel to already be expired")
	}

	// Deleting the base credential proves Activate never touches it again -- only the already-wrapped per-channel copy.
	if err := svc.creds.Delete("app-backend"); err != nil {
		t.Fatalf("deleting base credential: %v", err)
	}

	if _, err := svc.Activate(testRootSecret, c.ID, 5*time.Minute); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	_, err = svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees")
	wantsExecution(t, err)
}

func TestServiceQueryFailsWhenNeverActivated(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// Simulate a vault restart: metadata and the wrapped copy persist on disk, but the in-memory key cache is gone.
	svc.mu.Lock()
	delete(svc.activeKeys, c.ID)
	svc.mu.Unlock()

	if _, err := svc.Query(c.ID, []string{"employees"}, "SELECT id FROM employees"); err == nil {
		t.Error("Query on a channel with no cached key returned nil error")
	}
}

func TestServiceCreateChannelWrongRootSecretFails(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	if _, err := svc.CreateChannel("wrong passphrase", "test-channel", "app-backend", fullScope(), 5*time.Minute); err == nil {
		t.Error("CreateChannel with the wrong root secret returned nil error")
	}
}

func TestServiceChannelScopeReadableWithoutActivation(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
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
	if got.DBName != "app-backend" {
		t.Errorf("ChannelScope.DBName = %q, want app-backend", got.DBName)
	}
}

// wantsListingTables asserts err is the "reached the query" failure listTables reports for an
// unreachable connection -- proof ListTables got past decrypt/open before failing.
func wantsListingTables(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("ListTables against an unreachable database returned nil error")
	}
	if !strings.Contains(err.Error(), "vault: listing tables") {
		t.Errorf("ListTables error = %q, want it to fail at the listing query, not earlier", err.Error())
	}
}

func TestServiceListTablesReachesQueryAfterDecrypt(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	_, err := svc.ListTables(testRootSecret, "app-backend")
	wantsListingTables(t, err)
}

func TestServiceListTablesWrongRootSecretFails(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	if _, err := svc.ListTables("wrong passphrase", "app-backend"); err == nil {
		t.Error("ListTables with the wrong root secret returned nil error")
	}
}

func TestServiceListChannels(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte("creds")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	empty, err := svc.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels on an empty vault: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListChannels on an empty vault = %v, want none", empty)
	}

	a, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel a: %v", err)
	}
	b, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
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

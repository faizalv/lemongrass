package vault

import (
	"strings"
	"testing"
	"time"
)

const editedConnString = "mysql://root:rotated@127.0.0.1:2/appdb"

func wrappedCopy(t *testing.T, svc *Service, c Channel) string {
	t.Helper()
	key, err := DeriveKey(testRootSecret, c.Salt)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := svc.channels.Get(string(c.ID), key)
	if err != nil {
		t.Fatalf("reading wrapped copy of %s: %v", c.ID, err)
	}
	return string(plain)
}

func TestGetConnectionReturnsStoredString(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetConnection(testRootSecret, "app-backend")
	if err != nil || got != unreachableConnString {
		t.Errorf("GetConnection = %q, %v, want the stored string", got, err)
	}
	if _, err := svc.GetConnection("wrong passphrase", "app-backend"); err == nil {
		t.Error("GetConnection accepted a wrong passphrase")
	}
	if _, err := svc.GetConnection(testRootSecret, "missing"); err == nil {
		t.Error("GetConnection returned nil error for an unknown name")
	}
}

func TestUpdateConnectionRewrapsActiveAndInactiveChannelsAndDropsCachedDB(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	active, err := svc.CreateChannel(testRootSecret, "active", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := svc.CreateChannel(testRootSecret, "inactive", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	svc.forgetKey(inactive.ID)

	_, err = svc.Query(active.ID, []string{"employees"}, "SELECT id FROM employees")
	wantsExecution(t, err)
	svc.mu.Lock()
	_, cached := svc.dbConns[active.ID]
	svc.mu.Unlock()
	if !cached {
		t.Fatal("expected the baseline query to cache a database handle")
	}

	if err := svc.UpdateConnection(testRootSecret, "app-backend", editedConnString); err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}

	svc.mu.Lock()
	_, cached = svc.dbConns[active.ID]
	svc.mu.Unlock()
	if cached {
		t.Error("the active channel still holds a database handle opened with the old credential")
	}
	if got := wrappedCopy(t, svc, active); got != editedConnString {
		t.Errorf("active channel copy = %q, want the edited string", got)
	}
	if got := wrappedCopy(t, svc, inactive); got != editedConnString {
		t.Errorf("inactive channel copy = %q, want the edited string", got)
	}
	if got, err := svc.GetConnection(testRootSecret, "app-backend"); err != nil || got != editedConnString {
		t.Errorf("stored connection = %q, %v, want the edited string", got, err)
	}

	_, err = svc.Query(active.ID, []string{"employees"}, "SELECT id FROM employees")
	wantsExecution(t, err)
	if _, err := svc.Activate(testRootSecret, inactive.ID, 5*time.Minute); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	_, err = svc.Query(inactive.ID, []string{"employees"}, "SELECT id FROM employees")
	wantsExecution(t, err)

	if c, err := svc.ChannelScope(active.ID); err != nil || c.Port != active.Port || c.Name != "active" {
		t.Errorf("channel metadata = %+v, %v, want it unchanged", c, err)
	}
}

func TestUpdateConnectionWrongPassphraseChangesNothing(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateChannel(testRootSecret, "test-channel", "app-backend", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateConnection("wrong passphrase", "app-backend", editedConnString); err == nil {
		t.Fatal("expected an error for a wrong passphrase")
	}
	if got, _ := svc.GetConnection(testRootSecret, "app-backend"); got != unreachableConnString {
		t.Errorf("stored connection = %q, want it unchanged", got)
	}
	if got := wrappedCopy(t, svc, c); got != unreachableConnString {
		t.Errorf("channel copy = %q, want it unchanged", got)
	}
}

func TestUpdateConnectionUnknownName(t *testing.T) {
	svc := openTestService(t)
	if err := svc.UpdateConnection(testRootSecret, "missing", editedConnString); err == nil {
		t.Error("expected an error updating a connection that was never stored")
	}
	if names, _ := svc.ListConnections(); len(names) != 0 {
		t.Errorf("connections = %v, want none created by a failed update", names)
	}
}

func TestUpdateConnectionRefusesEngineChange(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	err := svc.UpdateConnection(testRootSecret, "app-backend", "postgres://root:secret@127.0.0.1:1/appdb")
	if err == nil || !strings.Contains(err.Error(), "engine cannot change") {
		t.Errorf("err = %v, want an engine-change refusal", err)
	}
	if got, _ := svc.GetConnection(testRootSecret, "app-backend"); got != unreachableConnString {
		t.Errorf("stored connection = %q, want it unchanged", got)
	}

	if err := svc.UpdateConnection(testRootSecret, "app-backend", "mariadb://root:secret@127.0.0.1:2/appdb"); err != nil {
		t.Errorf("mysql to mariadb is the same engine, got %v", err)
	}
	if err := svc.UpdateConnection(testRootSecret, "app-backend", "no-scheme"); err == nil {
		t.Error("expected an error for a connection string with no scheme")
	}
}

func TestUpdateConnectionLeavesOtherConnectionsChannelsAlone(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	if err := svc.PutCredential(testRootSecret, "other", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateChannel(testRootSecret, "on-other", "other", fullScope(), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateConnection(testRootSecret, "app-backend", editedConnString); err != nil {
		t.Fatal(err)
	}
	if got := wrappedCopy(t, svc, c); got != unreachableConnString {
		t.Errorf("channel on another connection = %q, want it untouched", got)
	}
}

func TestIPCGetAndUpdateConnectionRoundTrip(t *testing.T) {
	client := startTestServer(t)
	if err := client.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatal(err)
	}

	got, err := client.GetConnection(testRootSecret, "app-backend")
	if err != nil || got != unreachableConnString {
		t.Fatalf("GetConnection = %q, %v", got, err)
	}
	if err := client.UpdateConnection(testRootSecret, "app-backend", editedConnString); err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	again, err := client.GetConnection(testRootSecret, "app-backend")
	if err != nil || again != editedConnString {
		t.Errorf("after UpdateConnection: %q, %v, want the edited string", again, err)
	}
	if _, err := client.GetConnection("wrong-secret", "app-backend"); err == nil {
		t.Error("GetConnection accepted a wrong passphrase")
	}
}

package vault

import (
	"database/sql"
	"os"
	"testing"
	"time"
)

// TestServiceQueryAgainstRealMySQL exercises the whole Phase 3 path -- classify, declared-vs-
// actual, scope, connection caching, execution, row normalization -- against a real server
// instead of the unreachable-port trick the rest of this package's tests use. It needs
// LGRASS_TEST_MYSQL_DSN set to a mysql://user:pass@host:port/db connection string reachable
// from this machine (a local docker mysql works); it's skipped otherwise.
func TestServiceQueryAgainstRealMySQL(t *testing.T) {
	dsn := os.Getenv("LGRASS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("LGRASS_TEST_MYSQL_DSN not set, skipping real-MySQL test")
	}

	setupDSN, err := mysqlDSN(dsn)
	if err != nil {
		t.Fatalf("mysqlDSN: %v", err)
	}
	setup, err := sql.Open("mysql", setupDSN)
	if err != nil {
		t.Fatalf("opening setup connection: %v", err)
	}
	defer setup.Close()

	if _, err := setup.Exec(`DROP TABLE IF EXISTS lg_test_employees`); err != nil {
		t.Fatalf("dropping lg_test_employees: %v", err)
	}
	if _, err := setup.Exec(`CREATE TABLE lg_test_employees (id INT, name VARCHAR(64), manager VARCHAR(64) NULL)`); err != nil {
		t.Fatalf("creating lg_test_employees: %v", err)
	}
	t.Cleanup(func() { setup.Exec(`DROP TABLE IF EXISTS lg_test_employees`) })
	if _, err := setup.Exec(`INSERT INTO lg_test_employees (id, name, manager) VALUES (1, 'Alice', NULL), (2, 'Bob', 'Alice')`); err != nil {
		t.Fatalf("seeding lg_test_employees: %v", err)
	}

	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "kencana-backend", []byte(dsn)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	scope := Scope{Tables: []string{"lg_test_employees"}, Operations: []string{"select", "show"}}
	c, err := svc.CreateChannel(testRootSecret, "kencana-backend", scope, 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	result, err := svc.Query(c.ID, []string{"lg_test_employees"}, "SELECT id, name, manager FROM lg_test_employees ORDER BY id")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	wantColumns := []string{"id", "name", "manager"}
	if len(result.Columns) != len(wantColumns) {
		t.Fatalf("Columns = %v, want %v", result.Columns, wantColumns)
	}
	for i, c := range wantColumns {
		if result.Columns[i] != c {
			t.Errorf("Columns[%d] = %q, want %q", i, result.Columns[i], c)
		}
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if result.Rows[0][1] != "Alice" || result.Rows[0][2] != nil {
		t.Errorf("Rows[0] = %v, want [.. Alice <nil>]", result.Rows[0])
	}
	if result.Rows[1][1] != "Bob" || result.Rows[1][2] != "Alice" {
		t.Errorf("Rows[1] = %v, want [.. Bob Alice]", result.Rows[1])
	}

	// Calling Query again should reuse the cached *sql.DB rather than opening a second connection.
	if _, err := svc.Query(c.ID, []string{"lg_test_employees"}, "SELECT id FROM lg_test_employees"); err != nil {
		t.Fatalf("second Query: %v", err)
	}
	svc.mu.Lock()
	numConns := len(svc.dbConns)
	svc.mu.Unlock()
	if numConns != 1 {
		t.Errorf("len(dbConns) = %d, want 1 cached connection", numConns)
	}

	showResult, err := svc.Query(c.ID, []string{"*"}, "SHOW TABLES")
	if err != nil {
		t.Fatalf("SHOW TABLES: %v", err)
	}
	if len(showResult.Rows) == 0 {
		t.Error("SHOW TABLES returned no rows")
	}

	if err := svc.Revoke(c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	svc.mu.Lock()
	numConns = len(svc.dbConns)
	svc.mu.Unlock()
	if numConns != 0 {
		t.Errorf("len(dbConns) after Revoke = %d, want 0", numConns)
	}
}

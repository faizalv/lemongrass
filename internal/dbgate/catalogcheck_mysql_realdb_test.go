// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package dbgate

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/vault"
)

// Needs the vaultplain user from scripts/dbgate-realdb, which cannot read performance_schema.

func TestFunctionCheckWithoutPerformanceSchema(t *testing.T) {
	dsn := os.Getenv("LGRASS_TEST_MYSQL_PLAIN_DSN")
	if dsn == "" {
		t.Skip("LGRASS_TEST_MYSQL_PLAIN_DSN not set, skipping real-MySQL test")
	}

	raw := openRealDB(t, dsn)
	if _, err := raw.Exec("SELECT 1 FROM performance_schema.user_defined_functions LIMIT 0"); !mysqlLoadableDenied(err) {
		t.Fatalf("the plain user must be denied user_defined_functions, got %v", err)
	}

	svc := openTestService(t)
	notice, err := svc.Gate.TestConnection(dsn)
	if err != nil || notice == nil || notice.Fix != "GRANT SELECT ON performance_schema.user_defined_functions TO 'vaultplain'@'%';" {
		t.Errorf("TestConnection = %+v, %v, want a passing test naming the missing grant", notice, err)
	}
	if full := os.Getenv("LGRASS_TEST_MYSQL_DSN"); full != "" {
		if notice, err := svc.Gate.TestConnection(full); err != nil || notice != nil {
			t.Errorf("TestConnection for the app user = %+v, %v, want no notice", notice, err)
		}
	}

	if err := svc.PutCredential(testRootSecret, "plain-db", []byte(dsn)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	ch, err := svc.CreateChannel(testRootSecret, "plain", "plain-db", vault.Scope{Tables: []string{"employees", "salaries"}, Operations: []string{"select"}}, 10*time.Minute)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	for _, q := range []string{
		"SELECT COUNT(*), MIN(amount), MAX(amount) FROM salaries",
		"SELECT CONCAT(name, '!'), UPPER(name), DATE(NOW()) FROM employees",
	} {
		if _, err := svc.Query(ch.ID, q); err != nil {
			t.Errorf("Query(%q) = %v, want it to run on the ROUTINES-only check", q, err)
		}
	}

	_, err = svc.Query(ch.ID, "SELECT sum(id) FROM employees")
	var notAllowed *ErrFunctionNotAllowed
	if !errors.As(err, &notAllowed) || notAllowed.Name != "sum" {
		t.Errorf("stored sum = %v, want the ROUTINES half to refuse it", err)
	}
}

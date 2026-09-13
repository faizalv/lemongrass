package vault

import (
	"reflect"
	"testing"
)

func TestClassifyQueryMySQLSelect(t *testing.T) {
	cases := []struct {
		name   string
		sql    string
		tables []string
	}{
		{"single table", "SELECT * FROM employees", []string{"employees"}},
		{"quoted identifier", "select * from `employees`", []string{"employees"}},
		{"join, alias not counted as a table", "SELECT e.id, s.amount FROM employees e JOIN salaries s ON e.id = s.emp_id", []string{"employees", "salaries"}},
		{"subquery in where", "SELECT * FROM employees WHERE id IN (SELECT emp_id FROM salaries)", []string{"employees", "salaries"}},
		{"subquery in select list", "SELECT (SELECT COUNT(*) FROM salaries) AS n FROM employees", []string{"employees", "salaries"}},
		{"union", "SELECT id FROM employees UNION SELECT id FROM contractors", []string{"contractors", "employees"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ClassifyQuery(EngineMySQL, c.sql)
			if err != nil {
				t.Fatalf("ClassifyQuery: %v", err)
			}
			if got.Kind != KindSelect {
				t.Errorf("Kind = %q, want %q", got.Kind, KindSelect)
			}
			if !reflect.DeepEqual(got.Tables, c.tables) {
				t.Errorf("Tables = %v, want %v", got.Tables, c.tables)
			}
		})
	}
}

func TestClassifyQueryMySQLIntrospect(t *testing.T) {
	tests := []struct {
		sql        string
		wantTables []string
	}{
		{"SHOW TABLES", nil},
		{"SHOW COLUMNS FROM employees", []string{"employees"}},
		{"SHOW CREATE TABLE employees", []string{"employees"}},
		{"DESCRIBE employees", []string{"employees"}},
		{"DESC employees", []string{"employees"}},
		{"EXPLAIN SELECT * FROM employees", nil},
	}
	for _, tt := range tests {
		got, err := ClassifyQuery(EngineMySQL, tt.sql)
		if err != nil {
			t.Errorf("ClassifyQuery(%q): %v", tt.sql, err)
			continue
		}
		if got.Kind != KindIntrospect {
			t.Errorf("ClassifyQuery(%q).Kind = %q, want %q", tt.sql, got.Kind, KindIntrospect)
		}
		if !reflect.DeepEqual(got.Tables, tt.wantTables) {
			t.Errorf("ClassifyQuery(%q).Tables = %v, want %v", tt.sql, got.Tables, tt.wantTables)
		}
	}
}

func TestClassifyQueryMySQLRejectsWritesAndDDL(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO employees (name) VALUES ('a')",
		"UPDATE employees SET name = 'a' WHERE id = 1",
		"DELETE FROM employees WHERE id = 1",
		"DROP TABLE employees",
		"CREATE TABLE x (id INT)",
		"REPAIR TABLE employees",
	} {
		if _, err := ClassifyQuery(EngineMySQL, sql); err == nil {
			t.Errorf("ClassifyQuery(%q): expected rejection, got nil error", sql)
		}
	}
}

func TestClassifyQueryMySQLRejectsMultipleStatements(t *testing.T) {
	if _, err := ClassifyQuery(EngineMySQL, "SELECT 1; DROP TABLE employees"); err == nil {
		t.Error("expected stacked statements to be rejected")
	}
}

func TestClassifyQueryMySQLAllowsTrailingSemicolon(t *testing.T) {
	got, err := ClassifyQuery(EngineMySQL, "SELECT * FROM employees;")
	if err != nil {
		t.Fatalf("ClassifyQuery: %v", err)
	}
	if got.Kind != KindSelect || !reflect.DeepEqual(got.Tables, []string{"employees"}) {
		t.Errorf("got %+v", got)
	}
}

func TestClassifyQueryPostgresSelect(t *testing.T) {
	cases := []struct {
		name   string
		sql    string
		tables []string
	}{
		{"single table", `SELECT * FROM employees`, []string{"employees"}},
		{"join, alias not counted as a table", `SELECT e.id, s.amount FROM employees e JOIN salaries s ON e.id = s.emp_id`, []string{"employees", "salaries"}},
		{"subquery in where", `SELECT * FROM employees WHERE id IN (SELECT emp_id FROM salaries)`, []string{"employees", "salaries"}},
		{
			"CTE alias excluded, real tables kept",
			`WITH recent AS (SELECT id FROM employees WHERE active) SELECT r.id, s.amount FROM recent r JOIN salaries s ON r.id = s.emp_id`,
			[]string{"employees", "salaries"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ClassifyQuery(EnginePostgres, c.sql)
			if err != nil {
				t.Fatalf("ClassifyQuery: %v", err)
			}
			if got.Kind != KindSelect {
				t.Errorf("Kind = %q, want %q", got.Kind, KindSelect)
			}
			if !reflect.DeepEqual(got.Tables, c.tables) {
				t.Errorf("Tables = %v, want %v", got.Tables, c.tables)
			}
		})
	}
}

func TestClassifyQueryPostgresExplainKeepsWrappedTables(t *testing.T) {
	got, err := ClassifyQuery(EnginePostgres, "EXPLAIN SELECT * FROM employees JOIN salaries ON employees.id = salaries.emp_id")
	if err != nil {
		t.Fatalf("ClassifyQuery: %v", err)
	}
	if got.Kind != KindExplain {
		t.Errorf("Kind = %q, want %q", got.Kind, KindExplain)
	}
	if !reflect.DeepEqual(got.Tables, []string{"employees", "salaries"}) {
		t.Errorf("Tables = %v, want [employees salaries]", got.Tables)
	}
}

func TestClassifyQueryPostgresShow(t *testing.T) {
	got, err := ClassifyQuery(EnginePostgres, "SHOW search_path")
	if err != nil {
		t.Fatalf("ClassifyQuery: %v", err)
	}
	if got.Kind != KindIntrospect {
		t.Errorf("Kind = %q, want %q", got.Kind, KindIntrospect)
	}
}

func TestClassifyQueryPostgresRejectsWritesAndDDL(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO employees (name) VALUES ('a')",
		"UPDATE employees SET name = 'a' WHERE id = 1",
		"DELETE FROM employees WHERE id = 1",
		"DROP TABLE employees",
		"CREATE TABLE x (id INT)",
	} {
		if _, err := ClassifyQuery(EnginePostgres, sql); err == nil {
			t.Errorf("ClassifyQuery(%q): expected rejection, got nil error", sql)
		}
	}
}

func TestClassifyQueryPostgresRejectsMultipleStatements(t *testing.T) {
	if _, err := ClassifyQuery(EnginePostgres, "SELECT 1; DROP TABLE employees"); err == nil {
		t.Error("expected stacked statements to be rejected")
	}
}

func TestClassifyQueryPostgresAllowsTrailingSemicolon(t *testing.T) {
	got, err := ClassifyQuery(EnginePostgres, "SELECT * FROM employees;")
	if err != nil {
		t.Fatalf("ClassifyQuery: %v", err)
	}
	if got.Kind != KindSelect || !reflect.DeepEqual(got.Tables, []string{"employees"}) {
		t.Errorf("got %+v", got)
	}
}

func TestClassifyQueryRejectsUnparseableSQL(t *testing.T) {
	if _, err := ClassifyQuery(EngineMySQL, "SELEKT * FORM employees"); err == nil {
		t.Error("expected a parse error for garbage SQL")
	}
	if _, err := ClassifyQuery(EnginePostgres, "SELEKT * FORM employees"); err == nil {
		t.Error("expected a parse error for garbage SQL")
	}
}

func TestCheckDeclaredTablesAcceptsExactMatch(t *testing.T) {
	stmt := Statement{Kind: KindSelect, Tables: []string{"employees", "salaries"}}
	if err := checkDeclaredTables(stmt, []string{"salaries", "employees"}); err != nil {
		t.Errorf("checkDeclaredTables with the same tables in a different order: %v", err)
	}
}

func TestCheckDeclaredTablesRejectsMissingTable(t *testing.T) {
	stmt := Statement{Kind: KindSelect, Tables: []string{"employees", "salaries"}}
	if err := checkDeclaredTables(stmt, []string{"employees"}); err == nil {
		t.Error("checkDeclaredTables under-declaring a joined table returned nil, want ErrTableMismatch")
	}
}

func TestCheckDeclaredTablesRejectsExtraTable(t *testing.T) {
	stmt := Statement{Kind: KindSelect, Tables: []string{"employees"}}
	if err := checkDeclaredTables(stmt, []string{"employees", "salaries"}); err == nil {
		t.Error("checkDeclaredTables over-declaring a table the statement doesn't touch returned nil, want ErrTableMismatch")
	}
}

func TestCheckDeclaredTablesIntrospectRequiresWildcard(t *testing.T) {
	stmt := Statement{Kind: KindIntrospect}
	if err := checkDeclaredTables(stmt, []string{"*"}); err != nil {
		t.Errorf("checkDeclaredTables(introspect, [*]): %v", err)
	}
	if err := checkDeclaredTables(stmt, []string{"employees"}); err == nil {
		t.Error("checkDeclaredTables(introspect, [employees]) returned nil, want ErrTableMismatch")
	}
}

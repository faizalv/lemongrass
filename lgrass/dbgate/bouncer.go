package dbgate

import (
	"fmt"

	"github.com/faizalv/lemongrass/vault"
)

// ErrScopeViolation reports a channel scope denying a statement's kind or a specific table, with Table left empty for a table-less denial.
type ErrScopeViolation struct {
	Table     string
	Operation string
}

func (e *ErrScopeViolation) Error() string {
	if e.Table == "" {
		return fmt.Sprintf("dbgate: channel scope does not permit %s", e.Operation)
	}
	return fmt.Sprintf("dbgate: channel scope does not permit %s on %s", e.Operation, e.Table)
}

// ErrRestrictedColumn reports a statement that names, or selects with a star that would include, a column the vault never returns from a performance view.
type ErrRestrictedColumn struct {
	Table  string
	Column string
	Star   bool
}

func (e *ErrRestrictedColumn) Error() string {
	if e.Star {
		return fmt.Sprintf("dbgate: %s cannot be read with * because its %s column is not readable, select the other columns by name", e.Table, e.Column)
	}
	return fmt.Sprintf("dbgate: the %s column of %s is not readable", e.Column, e.Table)
}

// ErrTableMismatch reports the caller's declared tables not matching what the statement actually references, distinct from a scope denial.
type ErrTableMismatch struct {
	Declared []string
	Actual   []string
}

func (e *ErrTableMismatch) Error() string {
	return fmt.Sprintf("dbgate: declared tables %v, statement actually references %v", e.Declared, e.Actual)
}

// AllowStatement denies by default and checks every table a statement references against scope's Tables or, with the performance operation, the engine's view allowlist, skipping that check only for introspection with no recoverable table.
func AllowStatement(s vault.Scope, stmt Statement) error {
	op := string(stmt.Kind)
	if stmt.Kind == KindIntrospect {
		op = "show"
	}
	if !contains(s.Operations, op) {
		return &ErrScopeViolation{Operation: op}
	}
	performance := contains(s.Operations, OperationPerformance)
	for _, t := range stmt.Tables {
		if contains(s.Tables, t) {
			continue
		}
		if _, ok := perfViewRestricted(stmt.Engine, t); ok && performance {
			continue
		}
		return &ErrScopeViolation{Table: t, Operation: op}
	}
	return nil
}

// FilterTableList drops every row of a SHOW TABLES result naming a table outside s.Tables, since AllowStatement itself never scope-checks that statement's rows.
func FilterTableList(s vault.Scope, result QueryResult) QueryResult {
	allowed := toSet(s.Tables)
	rows := make([][]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		name, ok := row[0].(string)
		if ok && !allowed[name] {
			continue
		}
		rows = append(rows, row)
	}
	return QueryResult{Columns: result.Columns, Rows: rows}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

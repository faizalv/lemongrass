package vault

import "fmt"

// ErrScopeViolation reports a channel scope denying a statement's kind or one of its tables.
// Table is empty for a table-less (introspect) denial.
type ErrScopeViolation struct {
	Table     string
	Operation string
}

func (e *ErrScopeViolation) Error() string {
	if e.Table == "" {
		return fmt.Sprintf("vault: channel scope does not permit %s", e.Operation)
	}
	return fmt.Sprintf("vault: channel scope does not permit %s on %s", e.Operation, e.Table)
}

// ErrTableMismatch reports the caller's declared tables not matching what the statement
// actually references -- a distinct error from ErrScopeViolation, since a mismatch isn't
// necessarily a denial, just a wrong claim.
type ErrTableMismatch struct {
	Declared []string
	Actual   []string
}

func (e *ErrTableMismatch) Error() string {
	return fmt.Sprintf("vault: declared tables %v, statement actually references %v", e.Declared, e.Actual)
}

// AllowStatement denies by default: an empty Tables or Operations list permits nothing.
// A table-having statement (select/explain) needs its kind granted in Operations and every
// table it references granted in Tables. A table-less statement (introspect: SHOW/DESCRIBE,
// and MySQL's own EXPLAIN which its parser can't see inside) is never table-scope checked --
// it needs "show" granted in Operations instead.
func (s Scope) AllowStatement(stmt Statement) error {
	if stmt.Kind == KindIntrospect {
		if !contains(s.Operations, "show") {
			return &ErrScopeViolation{Operation: "show"}
		}
		return nil
	}
	if !contains(s.Operations, string(stmt.Kind)) {
		return &ErrScopeViolation{Operation: string(stmt.Kind)}
	}
	for _, t := range stmt.Tables {
		if !contains(s.Tables, t) {
			return &ErrScopeViolation{Table: t, Operation: string(stmt.Kind)}
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

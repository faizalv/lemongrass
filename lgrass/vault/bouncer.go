package vault

import "fmt"

type Query struct {
	Table     string
	Operation string
}

type ErrScopeViolation struct {
	Table     string
	Operation string
}

func (e *ErrScopeViolation) Error() string {
	return fmt.Sprintf("vault: channel scope does not permit %s on %s", e.Operation, e.Table)
}

// Allow denies by default: an empty Tables or Operations list permits nothing.
func (s Scope) Allow(q Query) error {
	if !contains(s.Tables, q.Table) || !contains(s.Operations, q.Operation) {
		return &ErrScopeViolation{Table: q.Table, Operation: q.Operation}
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

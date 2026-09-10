package vault

import "testing"

func TestScopeAllowPermitsGranted(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.Allow(Query{Table: "employees", Operation: "select"}); err != nil {
		t.Errorf("Allow on granted table/operation: %v", err)
	}
}

func TestScopeAllowDeniesUngrantedTable(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.Allow(Query{Table: "salaries", Operation: "select"}); err == nil {
		t.Error("Allow on an ungranted table returned nil, want ErrScopeViolation")
	}
}

func TestScopeAllowDeniesUngrantedOperation(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.Allow(Query{Table: "employees", Operation: "delete"}); err == nil {
		t.Error("Allow on an ungranted operation returned nil, want ErrScopeViolation")
	}
}

func TestScopeAllowDeniesEverythingWhenEmpty(t *testing.T) {
	var s Scope
	if err := s.Allow(Query{Table: "employees", Operation: "select"}); err == nil {
		t.Error("Allow on a zero-value Scope returned nil, want it to deny by default")
	}
}

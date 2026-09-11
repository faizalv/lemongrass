package vault

import "testing"

func TestAllowStatementPermitsGrantedSelect(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.AllowStatement(Statement{Kind: KindSelect, Tables: []string{"employees"}}); err != nil {
		t.Errorf("AllowStatement on a granted select: %v", err)
	}
}

func TestAllowStatementDeniesUngrantedTable(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.AllowStatement(Statement{Kind: KindSelect, Tables: []string{"salaries"}}); err == nil {
		t.Error("AllowStatement on an ungranted table returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesUngrantedKind(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.AllowStatement(Statement{Kind: KindExplain, Tables: []string{"employees"}}); err == nil {
		t.Error("AllowStatement on an ungranted kind returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesOneUngrantedTableAmongSeveral(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.AllowStatement(Statement{Kind: KindSelect, Tables: []string{"employees", "salaries"}}); err == nil {
		t.Error("AllowStatement with one ungranted table among several returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesEverythingWhenEmpty(t *testing.T) {
	var s Scope
	if err := s.AllowStatement(Statement{Kind: KindSelect, Tables: []string{"employees"}}); err == nil {
		t.Error("AllowStatement on a zero-value Scope returned nil, want it to deny by default")
	}
}

func TestAllowStatementIntrospectNeedsShowNotTables(t *testing.T) {
	s := Scope{Tables: nil, Operations: []string{"show"}}
	if err := s.AllowStatement(Statement{Kind: KindIntrospect}); err != nil {
		t.Errorf("AllowStatement on introspect with show granted: %v", err)
	}
}

func TestAllowStatementIntrospectDeniedWithoutShow(t *testing.T) {
	s := Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := s.AllowStatement(Statement{Kind: KindIntrospect}); err == nil {
		t.Error("AllowStatement on introspect without show granted returned nil, want ErrScopeViolation")
	}
}

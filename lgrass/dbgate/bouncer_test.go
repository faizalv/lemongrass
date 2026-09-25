package dbgate

import (
	"reflect"
	"testing"

	"github.com/faizalv/lemongrass/vault"
)

func TestAllowStatementPermitsGrantedSelect(t *testing.T) {
	s := vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := AllowStatement(s, Statement{Kind: KindSelect, Tables: []string{"employees"}}); err != nil {
		t.Errorf("AllowStatement on a granted select: %v", err)
	}
}

func TestAllowStatementDeniesUngrantedTable(t *testing.T) {
	s := vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := AllowStatement(s, Statement{Kind: KindSelect, Tables: []string{"salaries"}}); err == nil {
		t.Error("AllowStatement on an ungranted table returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesUngrantedKind(t *testing.T) {
	s := vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := AllowStatement(s, Statement{Kind: KindExplain, Tables: []string{"employees"}}); err == nil {
		t.Error("AllowStatement on an ungranted kind returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesOneUngrantedTableAmongSeveral(t *testing.T) {
	s := vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := AllowStatement(s, Statement{Kind: KindSelect, Tables: []string{"employees", "salaries"}}); err == nil {
		t.Error("AllowStatement with one ungranted table among several returned nil, want ErrScopeViolation")
	}
}

func TestAllowStatementDeniesEverythingWhenEmpty(t *testing.T) {
	var s vault.Scope
	if err := AllowStatement(s, Statement{Kind: KindSelect, Tables: []string{"employees"}}); err == nil {
		t.Error("AllowStatement on a zero-value vault.Scope returned nil, want it to deny by default")
	}
}

func TestAllowStatementIntrospectNeedsShowNotTables(t *testing.T) {
	s := vault.Scope{Tables: nil, Operations: []string{"show"}}
	if err := AllowStatement(s, Statement{Kind: KindIntrospect}); err != nil {
		t.Errorf("AllowStatement on introspect with show granted: %v", err)
	}
}

func TestAllowStatementIntrospectDeniedWithoutShow(t *testing.T) {
	s := vault.Scope{Tables: []string{"employees"}, Operations: []string{"select"}}
	if err := AllowStatement(s, Statement{Kind: KindIntrospect}); err == nil {
		t.Error("AllowStatement on introspect without show granted returned nil, want ErrScopeViolation")
	}
}

func TestFilterTableListDropsUngrantedTables(t *testing.T) {
	s := vault.Scope{Tables: []string{"menu", "sub_menu"}}
	result := QueryResult{
		Columns: []string{"Tables_in_db"},
		Rows: [][]interface{}{
			{"menu"}, {"sub_menu"}, {"users"}, {"mst_pemanen"},
		},
	}
	got := FilterTableList(s, result)
	want := [][]interface{}{{"menu"}, {"sub_menu"}}
	if !reflect.DeepEqual(got.Rows, want) {
		t.Errorf("FilterTableList rows = %v, want %v", got.Rows, want)
	}
}

func TestFilterTableListEmptyScopeDropsEverything(t *testing.T) {
	var s vault.Scope
	result := QueryResult{Columns: []string{"Tables_in_db"}, Rows: [][]interface{}{{"menu"}}}
	got := FilterTableList(s, result)
	if len(got.Rows) != 0 {
		t.Errorf("FilterTableList with empty scope returned %v, want no rows", got.Rows)
	}
}

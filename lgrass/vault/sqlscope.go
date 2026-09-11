package vault

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	// pgquery does the actual parsing (WASM-backed, no cgo); pganalyze supplies the parse
	// tree's own Go types, which pgquery.Parse returns values of.
	pganalyze "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
	"github.com/xwb1989/sqlparser"
)

// Kind classifies what a statement does, independent of engine.
type Kind string

const (
	KindSelect Kind = "select"
	// KindExplain wraps a query it doesn't itself execute; table-scope checked the same as
	// the wrapped statement's own tables, not a separate grant.
	KindExplain Kind = "explain"
	// KindIntrospect is SHOW/DESCRIBE-shaped: schema or server metadata, never row data.
	// Never table-scope checked -- gated purely by the channel's Operations.
	KindIntrospect Kind = "introspect"
)

// Statement is what the vault knows about a piece of SQL after classification: what kind
// of thing it does, and which real tables it references. Tables is the ground truth the
// vault checks scope against -- never the caller's own declared intent, which can be wrong
// or dishonest without this ever seeing it.
type Statement struct {
	Kind   Kind
	Tables []string
}

// ClassifyQuery rejects anything but a single statement of a supported read-only kind, and
// reports what it actually does. sqlText must be exactly one statement -- stacked
// statements are rejected outright, not silently truncated to the first.
func ClassifyQuery(engine Engine, sqlText string) (Statement, error) {
	switch engine {
	case EngineMySQL:
		return classifyMySQL(sqlText)
	case EnginePostgres:
		return classifyPostgres(sqlText)
	default:
		return Statement{}, fmt.Errorf("vault: unsupported engine %q", engine)
	}
}

func classifyMySQL(sqlText string) (Statement, error) {
	pieces, err := sqlparser.SplitStatementToPieces(sqlText)
	if err != nil {
		return Statement{}, fmt.Errorf("vault: splitting SQL: %w", err)
	}
	var stmts []string
	for _, p := range pieces {
		if strings.TrimSpace(p) != "" {
			stmts = append(stmts, p)
		}
	}
	if len(stmts) != 1 {
		return Statement{}, fmt.Errorf("vault: exactly one SQL statement is allowed, got %d", len(stmts))
	}

	stmt, err := sqlparser.Parse(stmts[0])
	if err != nil {
		return Statement{}, fmt.Errorf("vault: parsing SQL: %w", err)
	}

	switch stmt.(type) {
	case *sqlparser.Select, *sqlparser.Union, *sqlparser.ParenSelect:
		return Statement{Kind: KindSelect, Tables: mysqlTables(stmt)}, nil
	case *sqlparser.Show:
		return Statement{Kind: KindIntrospect}, nil
	case *sqlparser.OtherRead:
		// This parser's grammar routes exactly DESC/DESCRIBE/EXPLAIN here (see its sql.y),
		// swallowing the rest of the statement (force_eof) -- no table is ever recoverable
		// from this node, so these are always table-less introspection, same as SHOW.
		return Statement{Kind: KindIntrospect}, nil
	default:
		return Statement{}, fmt.Errorf("vault: %T is not a supported read-only statement", stmt)
	}
}

// mysqlTables walks stmt for AliasedTableExpr nodes whose Expr is a real TableName (not a
// subquery, which Walk recurses into separately). TableName is also reused elsewhere in
// this AST for a column reference's qualifier (e.g. the "e" in "e.id") -- collecting every
// TableName node in the tree, rather than only ones reached through a table position, would
// wrongly count aliases as tables.
func mysqlTables(stmt sqlparser.SQLNode) []string {
	seen := make(map[string]bool)
	sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		if ate, ok := node.(*sqlparser.AliasedTableExpr); ok {
			if tn, ok := ate.Expr.(sqlparser.TableName); ok && !tn.IsEmpty() {
				seen[tn.Name.String()] = true
			}
		}
		return true, nil
	}, stmt)
	return sortedKeys(seen)
}

func classifyPostgres(sqlText string) (Statement, error) {
	tree, err := pgquery.Parse(sqlText)
	if err != nil {
		return Statement{}, fmt.Errorf("vault: parsing SQL: %w", err)
	}
	if len(tree.Stmts) != 1 {
		return Statement{}, fmt.Errorf("vault: exactly one SQL statement is allowed, got %d", len(tree.Stmts))
	}

	var kind Kind
	switch tree.Stmts[0].Stmt.Node.(type) {
	case *pganalyze.Node_SelectStmt:
		kind = KindSelect
	case *pganalyze.Node_ExplainStmt:
		kind = KindExplain
	case *pganalyze.Node_VariableShowStmt:
		return Statement{Kind: KindIntrospect}, nil
	default:
		return Statement{}, fmt.Errorf("vault: %T is not a supported read-only statement", tree.Stmts[0].Stmt.Node)
	}

	tables, err := postgresTables(sqlText)
	if err != nil {
		return Statement{}, err
	}
	return Statement{Kind: kind, Tables: tables}, nil
}

// postgresTables re-parses sqlText to its JSON tree and walks it for every RangeVar's
// relname -- Postgres's grammar uses RangeVar exclusively for real relation references
// (FROM/JOIN sources), never for a column's alias qualifier, so no AliasedTableExpr-style
// disambiguation is needed here the way it is for the MySQL dialect. Names defined by the
// statement's own CTEs are excluded -- a WITH-clause alias isn't a real table, and requiring
// scope for a query's own invented name would be meaningless.
func postgresTables(sqlText string) ([]string, error) {
	js, err := pgquery.ParseToJSON(sqlText)
	if err != nil {
		return nil, fmt.Errorf("vault: parsing SQL: %w", err)
	}
	var tree any
	if err := json.Unmarshal([]byte(js), &tree); err != nil {
		return nil, fmt.Errorf("vault: decoding parsed SQL: %w", err)
	}

	ctes := make(map[string]bool)
	walkJSON(tree, func(key string, val any) {
		if key != "CommonTableExpr" {
			return
		}
		if m, ok := val.(map[string]any); ok {
			if name, ok := m["ctename"].(string); ok {
				ctes[name] = true
			}
		}
	})

	tables := make(map[string]bool)
	walkJSON(tree, func(key string, val any) {
		if key != "RangeVar" {
			return
		}
		if m, ok := val.(map[string]any); ok {
			if name, ok := m["relname"].(string); ok && !ctes[name] {
				tables[name] = true
			}
		}
	})
	return sortedKeys(tables), nil
}

// walkJSON depth-first visits every key/value pair in a tree decoded from JSON via
// json.Unmarshal into `any`.
func walkJSON(node any, visit func(key string, val any)) {
	switch v := node.(type) {
	case map[string]any:
		for k, val := range v {
			visit(k, val)
			walkJSON(val, visit)
		}
	case []any:
		for _, item := range v {
			walkJSON(item, visit)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkDeclaredTables compares the caller's declared table list, an audit-visible statement of
// intent, against what stmt actually references -- the ground truth. It is not a security
// boundary itself (AllowStatement is), just a clearer error than a bare scope denial when the
// two disagree. A table-less statement's only valid declaration is the literal "*".
func checkDeclaredTables(stmt Statement, declared []string) error {
	if stmt.Kind == KindIntrospect {
		if len(declared) == 1 && declared[0] == "*" {
			return nil
		}
		return &ErrTableMismatch{Declared: declared, Actual: nil}
	}
	want := sortedKeys(toSet(stmt.Tables))
	got := sortedKeys(toSet(declared))
	if !slices.Equal(want, got) {
		return &ErrTableMismatch{Declared: declared, Actual: stmt.Tables}
	}
	return nil
}

func toSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, v := range list {
		m[v] = true
	}
	return m
}

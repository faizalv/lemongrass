package vault

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	// pgquery parses via WASM (no cgo) into pganalyze's own parse-tree types.
	pganalyze "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
	"github.com/xwb1989/sqlparser"
)

// Kind classifies what a statement does, independent of engine.
type Kind string

const (
	KindSelect Kind = "select"
	// KindExplain is scope-checked against the tables of the query it wraps, not granted separately.
	KindExplain Kind = "explain"
	// KindIntrospect is SHOW/DESCRIBE-shaped schema or server metadata, table-scope checked only when a specific table is recoverable.
	KindIntrospect Kind = "introspect"
)

// Statement holds what a piece of SQL actually does and which real tables it references, the ground truth scope is checked against rather than the caller's declared intent.
type Statement struct {
	Kind   Kind
	Tables []string
	// ListsTables marks a SHOW TABLES-shaped statement, whose result rows name every table in the database rather than one this Statement itself references.
	ListsTables bool
}

// ClassifyQuery accepts exactly one statement of a supported read-only kind, rejecting stacked statements outright rather than truncating to the first.
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

	switch st := stmt.(type) {
	case *sqlparser.Select, *sqlparser.Union, *sqlparser.ParenSelect:
		return Statement{Kind: KindSelect, Tables: mysqlTables(stmt)}, nil
	case *sqlparser.Show:
		if st.HasOnTable() {
			return Statement{Kind: KindIntrospect, Tables: []string{st.OnTable.Name.String()}}, nil
		}
		if st.Type == "tables" {
			return Statement{Kind: KindIntrospect, ListsTables: true}, nil
		}
		return Statement{Kind: KindIntrospect, Tables: introspectTableFromText(stmts[0])}, nil
	case *sqlparser.OtherRead:
		return Statement{Kind: KindIntrospect, Tables: introspectTableFromText(stmts[0])}, nil
	default:
		return Statement{}, fmt.Errorf("vault: %T is not a supported read-only statement", stmt)
	}
}

// introspectTableRegex recovers the table this parser force_eofs out of DESCRIBE/DESC and SHOW CREATE TABLE/COLUMNS/FIELDS/INDEX/INDEXES/KEYS.
var introspectTableRegex = regexp.MustCompile(`(?is)^\s*(?:DESCRIBE|DESC|SHOW\s+CREATE\s+TABLE|SHOW\s+(?:COLUMNS|FIELDS|INDEX|INDEXES|KEYS)\s+(?:FROM|IN))\s+` + "`?([a-zA-Z0-9_$]+(?:\\.[a-zA-Z0-9_$]+)?)`?")

func introspectTableFromText(sqlText string) []string {
	m := introspectTableRegex.FindStringSubmatch(sqlText)
	if m == nil {
		return nil
	}
	return []string{m[1]}
}

// mysqlTables collects TableName nodes only via AliasedTableExpr, since TableName is reused for column qualifiers and walking it directly would count aliases as tables.
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

// postgresTables collects every RangeVar's relname except the statement's own CTE names, since Postgres uses RangeVar only for real relations, unlike MySQL's reused TableName.
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

// checkDeclaredTables compares the caller's declared tables against what the statement actually references, a clarity check only since AllowStatement is the real security boundary.
func checkDeclaredTables(stmt Statement, declared []string) error {
	if len(stmt.Tables) == 0 && stmt.Kind == KindIntrospect {
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

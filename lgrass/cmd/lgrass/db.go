package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/agent"
	"github.com/faizalv/lemongrass/vault"
)

const dbUsageBase = `usage: lgrass db <short-id> --tables <t1,t2|*> --sql "<statement>"

Runs a read-only statement (SELECT/SHOW/DESCRIBE/EXPLAIN) against the database a channel
grants access to. --tables is a declared statement of intent, checked against what the
statement actually references; it isn't the security boundary, the channel's own scope is.
Use * for a statement with no specific table (SHOW TABLES and the like).

A channel granted the performance operation can also SELECT from the performance views
below. Declare a MySQL view as schema.table, and quote names containing $ in backticks,
like sys.x$statement_analysis. Postgres views are declared by their bare name, or as
pg_catalog.name when the statement qualifies it. Columns holding raw statement text are
never readable: name the columns instead of using *, where a listed column is barred.
`

func dbUsageText() string {
	var b strings.Builder
	b.WriteString(dbUsageBase)
	for _, e := range []struct {
		label  string
		engine vault.Engine
	}{{"MySQL", vault.EngineMySQL}, {"Postgres", vault.EnginePostgres}} {
		fmt.Fprintf(&b, "\n%s performance views:\n  %s\n", e.label, strings.Join(vault.PerfViewNames(e.engine), "\n  "))
		restricted := vault.PerfViewRestrictedColumns(e.engine)
		names := make([]string, 0, len(restricted))
		for name := range restricted {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(&b, "  barred on %s: %s\n", name, strings.Join(restricted[name], ", "))
		}
	}
	return b.String()
}

func cmdDb(args []string) {
	if len(args) == 0 {
		fmt.Print(dbUsageText())
		return
	}

	shortID := args[0]
	var tables, sqlText string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--tables":
			i++
			if i < len(args) {
				tables = args[i]
			}
		case "--sql":
			i++
			if i < len(args) {
				sqlText = args[i]
			}
		}
	}
	if tables == "" || sqlText == "" {
		fmt.Fprint(os.Stderr, dbUsageText())
		os.Exit(1)
	}

	declaredTables := strings.Split(tables, ",")
	for i, t := range declaredTables {
		declaredTables[i] = strings.TrimSpace(t)
	}

	client := &agent.Client{SocketPath: agentSocketPath()}
	result, err := client.Query(shortID, declaredTables, sqlText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	out, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

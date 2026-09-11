package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/agent"
)

const dbUsage = `usage: lgrass db <short-id> --tables <t1,t2|*> --sql "<statement>"

Runs a read-only statement (SELECT/SHOW/DESCRIBE/EXPLAIN) against the database a channel
grants access to. --tables is a declared statement of intent, checked against what the
statement actually references; it isn't the security boundary, the channel's own scope is.
Use * for a statement with no specific table (SHOW TABLES and the like).
`

func cmdDb(args []string) {
	if len(args) == 0 {
		fmt.Print(dbUsage)
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
		fmt.Fprint(os.Stderr, dbUsage)
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

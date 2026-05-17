package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const helpText = `
migrategen — generate database migration files

USAGE
  migrategen --add-table   --name=<table>
  migrategen --drop-table  --name=<table>
  migrategen --alter-table --name=<table>

EXAMPLES
  migrategen --add-table --name=users
  migrategen --drop-table --name=users
  migrategen --alter-table --name=users

OUTPUT
  migrations/<timestamp>_<name>.{up,down}.sql
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stdout, helpText)
	}

	addTable := flag.Bool("add-table", false, "create table migration")
	dropTable := flag.Bool("drop-table", false, "drop table migration")
	alterTable := flag.Bool("alter-table", false, "alter table migration")
	name := flag.String("name", "", "table name")

	flag.Parse()

	if *name == "" {
		fmt.Println("table name is required")
		os.Exit(1)
	}

	actionCount := boolToInt(*addTable) + boolToInt(*dropTable) + boolToInt(*alterTable)
	if actionCount != 1 {
		fmt.Println("exactly one action flag is required")
		os.Exit(1)
	}

	migrationsDir := "migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		panic(err)
	}

	version := time.Now().Format("20060102150405")

	var upSQL, downSQL string
	kind := ""

	switch {
	case *addTable:
		kind = "add"
		upSQL = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id BIGSERIAL PRIMARY KEY
);
`, *name)

		downSQL = fmt.Sprintf(`DROP TABLE IF EXISTS %s;
`, *name)

	case *dropTable:
		kind = "drop"
		upSQL = fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, *name)

		createSQL, err := findCreateTableSQL(migrationsDir, *name)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		downSQL = createSQL
	case *alterTable:
		kind = "alter"
		upSQL = fmt.Sprintf(`ALTER TABLE %s
-- TODO: add alter statements;`, *name)

		downSQL = fmt.Sprintf(`-- TODO: rollback alter table %s
`, *name)
	}

	filename := fmt.Sprintf("%s_%s_%s", version, kind, *name)

	write(
		filepath.Join(migrationsDir, filename+".up.sql"),
		upSQL,
	)

	write(
		filepath.Join(migrationsDir, filename+".down.sql"),
		downSQL,
	)

	fmt.Println("migration created:", filename)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func write(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		panic(err)
	}
}

func findCreateTableSQL(dir, table string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var latest string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		sql := strings.ToLower(string(content))
		if strings.Contains(sql, "create table") && strings.Contains(sql, table) {
			latest = string(content)
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no create table migration found for %s", table)
	}

	return latest, nil
}

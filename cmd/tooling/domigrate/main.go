package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const helpText = `
domigrate — run database migrations

USAGE
  domigrate --cmd=up   --db=<database_url>
  domigrate --cmd=down --db=<database_url> [--step=N]

EXAMPLES
  domigrate --cmd=up --db=postgres://user:pass@localhost:5432/app?sslmode=disable
  domigrate --cmd=down --db=postgres://user:pass@localhost:5432/app?sslmode=disable --step=1

NOTES
  - migrations are read from ./migrations
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stdout, helpText)
	}

	cmd := flag.String("cmd", "", "migration command: up or down")
	dbURL := flag.String("db", "", "database url")
	step := flag.Int("step", 1, "migration steps (down only)")

	flag.Parse()

	if *cmd == "" || *dbURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	m, err := migrate.New(
		"file://migrations",
		*dbURL,
	)
	if err != nil {
		panic(err)
	}

	switch *cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			panic(err)
		}
		fmt.Println("migration up completed")

	case "down":
		if err := m.Steps(-*step); err != nil && err != migrate.ErrNoChange {
			panic(err)
		}
		fmt.Println("migration down completed")

	default:
		fmt.Println("invalid cmd:", *cmd)
		flag.Usage()
		os.Exit(1)
	}
}

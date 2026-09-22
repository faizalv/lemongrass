package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func usage() {
	fmt.Fprint(os.Stderr, `lgrassconf: keeps Lemongrass agent configuration in the state it needs

  run      Watches the config and repairs drift until stopped
  check    Runs one repair pass and exits
  install  Writes the systemd user unit and enables it
`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrassconf: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := newKeeper(home).run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "lgrassconf: %v\n", err)
			os.Exit(1)
		}
	case "check":
		if err := newKeeper(home).reconcile(); err != nil {
			fmt.Fprintf(os.Stderr, "lgrassconf: %v\n", err)
			os.Exit(1)
		}
	case "install":
		if err := installUnit(home); err != nil {
			fmt.Fprintf(os.Stderr, "lgrassconf: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
	}
}

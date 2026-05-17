package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/config"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "auth":
		cmdAuth()
	case "up":
		cmdUp()
	case "down":
		cmdDown()
	case "status":
		cmdStatus()
	case "fs-prep":
		cmdFsPrep()
	case "fs-daemon":
		cmdFsDaemon()
	case "remount":
		cmdRemount(os.Args[2:])
	case "_scaffold":
		config.EnsureScaffold()
		fmt.Println("~/.lemongrass/ initialized")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`lemongrass — Claude Code orchestrator

COMMANDS
  auth              Run Claude auth inside lg-runner container
  up                Start all containers
  down              Stop all containers
  status            Show container status
  fs-prep           Walk home directory and print all directory paths (used by server)
  remount <paths>   Recreate server container with given project paths mounted
`)
}

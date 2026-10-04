package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/cmd/lgrass/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(version.Version)
	case "--help", "-h", "help":
		usage()
	case "vault":
		cmdVault(os.Args[2:])
	case "agent":
		cmdAgent(os.Args[2:])
	case "tabs":
		cmdTabs(os.Args[2:])
	case "nudge":
		cmdNudge(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`lgrassd -- lemongrass's app-invoked binary

COMMANDS
  vault run                         Starts the credential-vault daemon, listening on a unix socket
                                     under ~/.lemongrass
  agent run                         Starts the gatekeeper agent daemon, listening on a unix socket
                                     under ~/.lemongrass; maps short channel ids to real vault
                                     channels and forwards queries to a running vault daemon
  tabs list                         Tab id to session id map for the project at the working directory, as JSON
  tabs register <tab-id> <vendor>   Records the agent vendor a tab runs
  tabs forget <tab-id>              Drops a closed tab's records
  nudge <tab-id>                    Prints the tab's pending thread nudge and marks it sent

  version                           Print version
`)
}

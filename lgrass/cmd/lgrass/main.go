package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/cmd/lgrass/version"
	"github.com/faizalv/lemongrass/project"
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
	case "init":
		cmdInit()
	case "hook":
		cmdHook(os.Args[2:])
	case "thread":
		cmdThread(os.Args[2:])
	case "session":
		cmdSession(os.Args[2:])
	case "vault":
		cmdVault(os.Args[2:])
	case "agent":
		cmdAgent(os.Args[2:])
	case "db":
		cmdDb(os.Args[2:])
	case "rester":
		cmdRester(os.Args[2:])
	case "tips":
		cmdTips(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "mention", "rules":
		notBuiltYet(os.Args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

// Identical to the Electron UI's own "add project" flow, so a bare terminal session can register without it.
func cmdInit() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	p, err := project.Register(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("lgrass: %s registered as project %q (%s)\n", p.Path, p.Name, p.ID)
}

func notBuiltYet(cmd string) {
	fmt.Fprintf(os.Stderr, "lgrass %s: not built yet\n", cmd)
	os.Exit(1)
}

func usage() {
	fmt.Print(`lgrass -- lemongrass's agent-invoked CLI

COMMANDS
  init                               Register the current directory as a lemongrass project

  hook <event>                      Invoked by Claude Code's own hook system, reads hook JSON off stdin.

  thread post "message" [--mention <session-id>]   Project-wide, not point-to-point; pushed live to other
                                     sessions' inbox sockets, and mentions surface via hook regardless
  thread list [--limit N]           Recent thread messages in this project, newest first
  thread listen [--timeout 10m]     Blocks until a new message arrives or the timeout elapses; meant to
                                     run backgrounded for a live channel, vendor neutral, relaunch on return

  mention <id> "comment"             Human-facing, opens a thread against the right session

  tips add "<message>"              Add a project-local custom tip, surfaced alongside the
                                     built-in ones on the periodic PostToolUse nudge
  tips list                         List this project's custom tips, with their ids
  tips remove <id>                  Remove a custom tip by id

  sign [--session-id <id>] <checklist-id>
                                     Satisfies a .lgrass/checklists.json prerequisite gate for the
                                     current session, until that checklist's TTL expires

  rules list
  rules add ...                     Human/UI-driven, not a model-facing write path

  session list                      Other live sessions in this project, with active/idling state

  vault run                         Starts the credential-vault daemon, listening on a unix socket
                                     under ~/.lemongrass; not model-facing, has no admin CLI yet

  agent run                         Starts the gatekeeper agent daemon, listening on a unix socket
                                     under ~/.lemongrass; maps short channel ids to real vault
                                     channels and forwards queries to a running vault daemon

  db <short-id> --tables <t1,t2|*> --sql "<statement>"
                                     Model-facing: runs a read-only statement (SELECT/SHOW/
                                     DESCRIBE/EXPLAIN) against the database a channel grants
                                     access to, through the running agent and vault daemons

  rester <short-id> info            Model-facing: prints a channel's base URL, expiry, allowed
                                     methods and users
  rester <short-id> users           Model-facing: prints a channel's users with their tags
  rester <short-id> <get|post|put|patch|delete|head|options> <path> [--user <name>] [--body '<json>']
                                     Model-facing: proxies one HTTP call through the channel's
                                     domain as user, through the running agent and vault daemons

  version                           Print version

mention and rules are not built yet.
`)
}

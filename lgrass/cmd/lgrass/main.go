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

	if err := ensureClaudeHooks(); err != nil {
		fmt.Fprintf(os.Stderr, "lgrass: registered the project, but failed to register Claude Code hooks: %v\n", err)
		os.Exit(1)
	}
}

func notBuiltYet(cmd string) {
	fmt.Fprintf(os.Stderr, "lgrass %s: not built yet\n", cmd)
	os.Exit(1)
}

func usage() {
	fmt.Print(`lgrass -- lemongrass's agent-invoked CLI

COMMANDS
  init                               Register the current directory as a lemongrass project, and
                                     register lgrass's Claude Code hooks (SessionStart/SessionEnd/
                                     PreToolUse/PostToolUse) in ~/.claude/settings.json if missing

  hook <event>                      Invoked by Claude Code's own hook system, reads hook JSON off stdin.

  thread post "message" [--mention <session-id>]   Project-wide, not point-to-point; pushed live to other
                                     sessions' inbox sockets, and mentions surface via hook regardless
  thread list [--limit N]           Recent thread messages in this project, newest first
  thread listen [--timeout 10m]     Blocks until a new message arrives or the timeout elapses; meant to
                                     run backgrounded for a live channel, vendor neutral, relaunch on return

  mention <id> "comment"             Human-facing, opens a thread against the right session

  rules list
  rules add ...                     Human/UI-driven, not a model-facing write path

  session list                      Other live sessions in this project, with active/idling state

  version                           Print version

mention and rules are not built yet.
`)
}

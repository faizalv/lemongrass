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
	case "knowledge":
		cmdKnowledge(os.Args[2:])
	case "hook":
		cmdHook(os.Args[2:])
	case "thread", "mention", "rules", "session":
		notBuiltYet(os.Args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
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
  knowledge write --tags a,b,c [--book <id> --chapter 2] [--title "..."] < body.md
  knowledge edit <id> --lines A-B < replacement.md   Targeted patch, not a full overwrite
  knowledge search "<query>"        Titles + tags + ids, not content; books collapse to one line
  knowledge read <id>               One entry's full body
  knowledge read --book <id>        Whole book, chapters in order
  knowledge reindex                 Rebuild the search index from disk
  knowledge book create --title "..." [--tags a,b] [--description "..."]
  knowledge toc                     Pointers only, for system-prompt injection

  hook <event>                      Invoked by Claude Code's own hook system, reads hook JSON off stdin

  thread open <session-id> "message"  Mints a thread id, delivers it, backgroundable
  thread reply <thread-id> "message"
  thread close <thread-id>
  thread list                       Threads open on / waiting on this session

  mention <knowledge-id> "comment"  Human-facing, opens a thread against the right session

  rules list
  rules add ...                     Human/UI-driven, not a model-facing write path

  session list                      Population + active/idling state, current project

  version                           Print version

thread, mention, rules, and session are not built yet.
`)
}

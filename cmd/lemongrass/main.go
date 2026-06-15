package main

import (
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/cmd/lemongrass/version"
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
	case "start-daemon":
		cmdStartDaemon()
	case "init":
		cmdInit(os.Args[2:])
	case "remount":
		cmdRemount(os.Args[2:])
	case "language":
		cmdLanguage(os.Args[2:])
	case "artifacts":
		cmdArtifacts(os.Args[2:])
	case "version":
		fmt.Println(version.Version)
	case "completion":
		cmdCompletion(os.Args[2:])
	case "update":
		cmdUpdate()
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
	fmt.Print(`lemongrass -- Claude Code orchestrator

COMMANDS
  auth                        Run Claude auth inside lg-runner container
  up                          Start all containers
  down                        Stop all containers
  status                      Show container status
  init [path]                 Register a project directory (default: current directory)
  remount <paths>             Recreate server container with given project paths mounted
  language add <lang...>      Enable one or more language parsers
  language remove <lang>      Disable a language parser
  language clear              Disable all language parsers
  language list               Show active and available language parsers
  artifacts export [path]     Export knowledge and annotations to a .lgart file
  artifacts import [flags] <path>  Import a .lgart file (--force, --dry-run)
  artifacts inspect <path>    Inspect a .lgart file for content and warnings
  completion <bash|zsh|fish>  Print shell completion script
  version                     Print version
  update                      Update to the latest release
`)
}

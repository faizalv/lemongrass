package main

import (
	"fmt"
	"os"
)

func cmdCompletion(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lemongrass completion <bash|zsh|fish>")
		os.Exit(1)
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s\nusage: lemongrass completion <bash|zsh|fish>\n", args[0])
		os.Exit(1)
	}
}

const bashCompletion = `_lemongrass_complete() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"

    local cmds="up down status auth init remount language artifacts completion version update"

    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        language)
            if [ $COMP_CWORD -eq 2 ]; then
                COMPREPLY=( $(compgen -W "add remove clear list" -- "$cur") )
            fi
            ;;
        artifacts)
            if [ $COMP_CWORD -eq 2 ]; then
                COMPREPLY=( $(compgen -W "export import inspect" -- "$cur") )
            fi
            ;;
        completion)
            if [ $COMP_CWORD -eq 2 ]; then
                COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            fi
            ;;
    esac
    return 0
}
complete -F _lemongrass_complete lemongrass
`

const zshCompletion = `#compdef lemongrass

_lemongrass() {
    local state
    _arguments \
        '1: :->cmd' \
        '*: :->args'

    case $state in
        cmd)
            local -a cmds
            cmds=(up down status auth init remount language artifacts completion version update)
            _describe 'command' cmds
            ;;
        args)
            case $words[2] in
                language)
                    local -a sub
                    sub=(add remove clear list)
                    _describe 'subcommand' sub
                    ;;
                artifacts)
                    local -a sub
                    sub=(export import inspect)
                    _describe 'subcommand' sub
                    ;;
                completion)
                    local -a shells
                    shells=(bash zsh fish)
                    _describe 'shell' shells
                    ;;
            esac
            ;;
    esac
}

_lemongrass "$@"
`

const fishCompletion = `set -l cmds up down status auth init remount language artifacts completion version update

complete -c lemongrass -f -n "not __fish_seen_subcommand_from $cmds" -a "$cmds"

complete -c lemongrass -f -n "__fish_seen_subcommand_from language" -a "add remove clear list"
complete -c lemongrass -f -n "__fish_seen_subcommand_from artifacts" -a "export import inspect"
complete -c lemongrass -f -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
`

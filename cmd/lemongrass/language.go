package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/config"
)

var supportedLangs = []string{"ts", "py", "php"}

func isSupportedLang(l string) bool {
	for _, s := range supportedLangs {
		if s == l {
			return true
		}
	}
	return false
}

func restartLgLang() {
	cfg := config.LoadOrDefault()
	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, queryProjectPaths()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update compose file: %v\n", err)
		return
	}
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "lg-lang")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not restart lg-lang: %v\n", err)
	}
}

func cmdLanguage(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass language <add|remove|clear|list> [lang...]\n")
		os.Exit(1)
	}
	switch args[0] {
	case "add":
		cmdLanguageAdd(args[1:])
	case "remove", "rm":
		cmdLanguageRemove(args[1:])
	case "clear":
		cmdLanguageClear()
	case "list":
		cmdLanguageList()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\nusage: lemongrass language <add|remove|clear|list>\n", args[0])
		os.Exit(1)
	}
}

func cmdLanguageAdd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass language add <lang> [lang...]\n")
		os.Exit(1)
	}

	// accept both space-separated and comma-separated for compat
	var langs []string
	for _, a := range args {
		for _, part := range strings.Split(a, ",") {
			l := strings.TrimSpace(strings.ToLower(part))
			if l == "" {
				continue
			}
			if l == "go" {
				fmt.Fprintln(os.Stderr, "Go parser is always active and cannot be added.")
				os.Exit(1)
			}
			if !isSupportedLang(l) {
				fmt.Fprintf(os.Stderr, "unknown language: %q\nsupported: %s\n", l, strings.Join(supportedLangs, ", "))
				os.Exit(1)
			}
			langs = append(langs, l)
		}
	}
	if len(langs) == 0 {
		fmt.Fprintln(os.Stderr, "no languages specified")
		os.Exit(1)
	}

	cfg := config.LoadOrDefault()
	existing := make(map[string]bool)
	for _, l := range cfg.Languages {
		existing[l] = true
	}
	var added []string
	for _, l := range langs {
		if !existing[l] {
			cfg.Languages = append(cfg.Languages, l)
			added = append(added, l)
		}
	}
	if len(added) == 0 {
		fmt.Println("nothing to add -- all specified languages already active")
		return
	}
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "language add: %v\n", err)
		os.Exit(1)
	}
	restartLgLang()
	fmt.Printf("Added: [%s]. Active: [%s]. lg-lang restarted.\n",
		strings.Join(added, " "), strings.Join(cfg.Languages, " "))
}

func cmdLanguageRemove(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass language remove <lang>\n")
		os.Exit(1)
	}
	target := strings.TrimSpace(strings.ToLower(args[0]))

	cfg := config.LoadOrDefault()
	var updated []string
	found := false
	for _, l := range cfg.Languages {
		if l == target {
			found = true
			continue
		}
		updated = append(updated, l)
	}
	if !found {
		fmt.Printf("%s is not active.\n", target)
		return
	}
	cfg.Languages = updated
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "language remove: %v\n", err)
		os.Exit(1)
	}
	restartLgLang()
	fmt.Printf("%s removed. lg-lang restarted.\n", target)
}

func cmdLanguageClear() {
	cfg := config.LoadOrDefault()
	cfg.Languages = []string{}
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "language clear: %v\n", err)
		os.Exit(1)
	}
	restartLgLang()
	fmt.Println("All language parsers cleared. lg-lang restarted.")
}

func cmdLanguageList() {
	cfg := config.LoadOrDefault()
	if len(cfg.Languages) == 0 {
		fmt.Println("No additional language parsers active. (Go is always active.)")
		return
	}
	fmt.Printf("Active: %s\n", strings.Join(cfg.Languages, " "))
	fmt.Printf("Available: %s\n", strings.Join(supportedLangs, " "))
}

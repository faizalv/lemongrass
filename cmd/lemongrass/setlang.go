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

func cmdSetLang(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass setlang <lang1,lang2,...> | --clear\n")
		os.Exit(1)
	}

	if args[0] == "--clear" {
		cfg := config.LoadOrDefault()
		cfg.Languages = []string{}
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "setlang: %v\n", err)
			os.Exit(1)
		}
		restartLgLang()
		fmt.Println("Language parsers cleared. lg-lang restarted.")
		return
	}

	parts := strings.Split(args[0], ",")
	var langs []string
	for _, p := range parts {
		l := strings.TrimSpace(strings.ToLower(p))
		if l == "" {
			continue
		}
		if l == "go" {
			fmt.Fprintln(os.Stderr, "Go parser is always active and cannot be set via setlang.")
			os.Exit(1)
		}
		if !isSupportedLang(l) {
			fmt.Fprintf(os.Stderr, "unknown language: %q\nsupported: %s\n", l, strings.Join(supportedLangs, ", "))
			os.Exit(1)
		}
		langs = append(langs, l)
	}
	if len(langs) == 0 {
		fmt.Fprintln(os.Stderr, "no languages specified")
		os.Exit(1)
	}

	cfg := config.LoadOrDefault()
	cfg.Languages = langs
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "setlang: %v\n", err)
		os.Exit(1)
	}
	restartLgLang()
	fmt.Printf("Language parsers set to: [%s]\nlg-lang restarted.\n", strings.Join(langs, " "))
}

func cmdRmLang(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass rmlang <lang>\n")
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
		fmt.Printf("%s is not installed.\n", target)
		return
	}
	cfg.Languages = updated
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "rmlang: %v\n", err)
		os.Exit(1)
	}
	restartLgLang()
	fmt.Printf("%s removed. lg-lang restarted.\n", target)
}

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

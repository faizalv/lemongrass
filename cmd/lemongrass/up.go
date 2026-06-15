package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/cmd/lemongrass/version"
	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/infra/lgprompt"
)

func cmdUp() {
	config.EnsureScaffold()
	config.DetectAndSaveDevice()

	credFile := filepath.Join(config.Dir(), "claude", ".credentials.json")
	info, err := os.Stat(credFile)
	if err != nil || info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "error: no credentials found. Run: lemongrass auth")
		os.Exit(1)
	}

	teardown()

	cfg := config.LoadOrDefault()
	if cfg.HomeDir == "" {
		cfg.HomeDir, _ = os.UserHomeDir()
	}
	if cfg.BinPath == "" {
		cfg.BinPath = config.DetectBinPath()
	}
	config.Save(cfg)

	writeHookSettings(cfg)
	installAndStartDaemon(cfg.BinPath)

	pullImages(version.Version)

	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, nil, version.Version), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write compose file: %v\n", err)
		os.Exit(1)
	}

	pgUp := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait", "lg-postgres")
	pgUp.Stdout = os.Stdout
	pgUp.Stderr = os.Stderr
	if err := pgUp.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres: %v\n", err)
		os.Exit(1)
	}

	projectPaths := queryProjectPaths()

	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, projectPaths, version.Version), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write compose file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start containers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Lemongrass is running at http://%s:%d\n", cfg.Host, cfg.Port)
	installHostHook()
}

func installHostHook() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not locate lemongrass binary, skipping lg-hook-host install")
		return
	}
	hookSrc := filepath.Join(filepath.Dir(exe), "lg-hook-host")
	if _, err := os.Stat(hookSrc); err != nil {
		fmt.Fprintln(os.Stderr, "warning: lg-hook-host not found next to lemongrass binary, skipping install")
		return
	}

	hookDst := filepath.Join(filepath.Dir(exe), "lg-hook-host")
	installDir := filepath.Dir(exe)
	localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if _, err := os.Stat(localBin); err == nil {
		hookDst = filepath.Join(localBin, "lg-hook-host")
		installDir = localBin
	} else {
		hookDst = "/usr/local/bin/lg-hook-host"
		installDir = "/usr/local/bin"
	}
	_ = installDir

	data, err := os.ReadFile(hookSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read lg-hook-host: %v\n", err)
		return
	}
	if err := os.WriteFile(hookDst, data, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not install lg-hook-host to %s: %v\n", hookDst, err)
		return
	}
	fmt.Printf("installed lg-hook-host -> %s\n", hookDst)

	mergeClaudeSettings(hookDst)
	writeSkillFile()
}

func mergeClaudeSettings(hookPath string) {
	home := os.Getenv("HOME")
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	var root map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &root)
	}
	if root == nil {
		root = map[string]any{}
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	entry := map[string]any{
		"type":    "command",
		"command": hookPath,
	}
	matchers := []string{"Bash", "Write", "Read", "Edit"}
	pre, _ := hooks["PreToolUse"].([]any)
	for _, m := range matchers {
		already := false
		for _, item := range pre {
			if h, ok := item.(map[string]any); ok {
				if h["matcher"] == m {
					for _, hk := range toSlice(h["hooks"]) {
						if c, ok := hk.(map[string]any); ok && c["command"] == hookPath {
							already = true
						}
					}
				}
			}
		}
		if !already {
			pre = append(pre, map[string]any{
				"matcher": m,
				"hooks":   []any{entry},
			})
		}
	}
	hooks["PreToolUse"] = pre

	postCompact, _ := hooks["PostCompact"].([]any)
	for _, matcher := range []string{"auto", "manual"} {
		already := false
		for _, item := range postCompact {
			if h, ok := item.(map[string]any); ok && h["matcher"] == matcher {
				for _, hk := range toSlice(h["hooks"]) {
					if c, ok := hk.(map[string]any); ok && c["command"] == hookPath {
						already = true
					}
				}
			}
		}
		if !already {
			postCompact = append(postCompact, map[string]any{
				"matcher": matcher,
				"hooks":   []any{entry},
			})
		}
	}
	hooks["PostCompact"] = postCompact

	postToolUse, _ := hooks["PostToolUse"].([]any)
	for _, m := range []string{"Write", "Edit", "Read", "Skill"} {
		already := false
		for _, item := range postToolUse {
			if h, ok := item.(map[string]any); ok && h["matcher"] == m {
				for _, hk := range toSlice(h["hooks"]) {
					if c, ok := hk.(map[string]any); ok && c["command"] == hookPath {
						already = true
					}
				}
			}
		}
		if !already {
			postToolUse = append(postToolUse, map[string]any{
				"matcher": m,
				"hooks":   []any{entry},
			})
		}
	}
	hooks["PostToolUse"] = postToolUse

	root["hooks"] = hooks

	data, _ := json.MarshalIndent(root, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write ~/.claude/settings.json: %v\n", err)
		return
	}
	fmt.Println("updated ~/.claude/settings.json")
}

func toSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func writeSkillFile() {
	home := os.Getenv("HOME")
	skillsDir := filepath.Join(home, ".claude", "skills", "lemongrass")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create ~/.claude/skills/lemongrass/: %v\n", err)
		return
	}
	skillPath := filepath.Join(skillsDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(lgprompt.BuildSkillContent()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write skill file: %v\n", err)
		return
	}
	fmt.Printf("wrote skill file -> %s\n", skillPath)
}

func pullImages(ver string) {
	if ver == "dev" {
		return
	}
	images := []string{"server", "runner", "embed", "lang"}
	refs := make([]string, len(images))
	for i, name := range images {
		refs[i] = "ghcr.io/faizalv/lemongrass/lg-" + name + ":v" + ver
	}

	fmt.Printf("Pulling images  (v%s)\n\n", ver)
	for _, ref := range refs {
		fmt.Printf("  %s  pulling...\n", ref)
		cmd := exec.Command("docker", "pull", ref)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\npull failed: %s: %v\n", ref, err)
			os.Exit(1)
		}
	}
	fmt.Println()
}

func queryProjectPaths() []string {
	out, err := exec.Command("docker", "exec", "lg-postgres",
		"psql", "-U", "lemongrass", "-tAc",
		"SELECT path FROM lg_projects WHERE status != 'removed'",
	).Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func writeHookSettings(cfg config.Config) {
	claudeDir := filepath.Join(cfg.HomeDir, ".lemongrass", "claude")
	if cfg.HomeDir == "" {
		claudeDir = filepath.Join(config.Dir(), "claude")
	}
	settings := `{
  "permissions": {
    "allow": ["Write", "Edit"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      },
      {
        "matcher": "Write",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      },
      {
        "matcher": "Read",
        "hooks": [{"type": "command", "command": "lg-hook"}]
      }
    ]
  }
}`
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0644)
}

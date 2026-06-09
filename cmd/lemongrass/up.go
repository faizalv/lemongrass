package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/config"
)

func cmdUp() {
	config.EnsureScaffold()

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

	composePath := filepath.Join(config.Dir(), "docker-compose.yml")
	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, nil), 0644); err != nil {
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

	if err := os.WriteFile(composePath, config.GenerateCompose(cfg, projectPaths), 0644); err != nil {
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
	matchers := []string{"Bash", "Write", "Read"}
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
	if err := os.WriteFile(skillPath, []byte(skillFileContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write skill file: %v\n", err)
		return
	}
	fmt.Printf("wrote skill file -> %s\n", skillPath)
}

const skillFileContent = `You are working in a lemongrass project. A live semantic map covers the codebase -- every function, method, type, and symbol is indexed with embeddings. Reach for the map first, not file reads or grep.

HOW TO FIND THINGS

You do not know the path -- start with semantic search:
  #lg.recon.search <query>

You know the path and want to see what symbols are there:
  #lg.recon.peek <path>

You know the exact symbol and want to read its body:
  #lg.recon.read <path:symbol:kind>

You want to know what calls a symbol or what it calls:
  #lg.recon.related <path:symbol:kind>

peek displays methods as Receiver.Method (LgUsecase.HandleByProject). All commands -- recon.read, recon.related, codebase.interim S: -- take the bare name (HandleByProject). The recon.read response header confirms the correct triple.

You want a coverage overview of the whole project or a subtree:
  #lg.recon.tree              (no path = full project entrance)
  #lg.recon.tree <path>

Search results carry a status marker. unexplored means the node has a provisional embedding from its signature -- description is empty. explored means a human-written description exists and the embedding reflects it. Reading and annotating unexplored nodes improves the map permanently for every future session.

WHEN YOU NEED FILE CONTENT

recon.read gives you a symbol body. When you need actual file bytes -- cross-cutting analysis, string patterns, reading around a symbol -- load the workbench first:

  #lg.codebase.interim <S:path:symbol | F:path | R:glob>   load files or symbols
  #lg.codebase.query <question>                            semantic search within workbench
  #lg.codebase.search <pattern>                            pattern search within workbench

Use recon.read when the symbol is the thing. Use codebase.interim when the file content around it is the thing.

KNOWLEDGE

Persist things that would cost another session time to re-derive: architectural decisions, module boundaries, non-obvious constraints, build procedures. Not task notes. Not things already readable from the code.

  #lg.knowledge.save <key>:<content> [label1,label2]
  #lg.knowledge.read <key>
  #lg.knowledge.search <query>
  #lg.knowledge.labels <query>          find relevant label names before saving
  #lg.knowledge.delete <key>

WORKSPACES AND TASKS

Workspaces record what you and the user agreed to do. Tasks track progress and build a readable history in the UI.

  #lg.workspace.create <name>
  #lg.workspace.use <name>
  #lg.tasks.checkpoint                  write down the agreed task list
  #lg.tasks.start <taskID>
  #lg.tasks.finish <taskID>:<notes>

Checkpoint writes down a conclusion already reached in conversation. Do not call it speculatively.

MODES

PTY mode -- you are running inside lg-runner, driven by the grooming pipeline. Call #lg.commitment <path> before annotating a directory. This registers your scope and gates the checkpoint. Read before annotating -- blind annotations do not count toward commitment.

  #lg.commitment <path>
  #lg.commitment.status

Headless mode -- you are Claude Code running on the host, using lemongrass as infrastructure. Commitment is not required. Annotate freely when you read something worth recording. Checkpoint and tasks work as records, not gates.
`

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

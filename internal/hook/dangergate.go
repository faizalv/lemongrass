package hook

import (
	"encoding/json"
	"os"

	"github.com/faizalv/lemongrass/internal/guard"
)

func dangerDecision(payload hookEvent) *guard.Verdict {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	input := guard.Input{Tool: payload.ToolName, Cwd: payload.Cwd, Home: home}
	if payload.ToolName == "Bash" {
		var shell shellToolInput
		if err := json.Unmarshal(payload.ToolInput, &shell); err != nil {
			return nil
		}
		input.Command = shell.Command
	} else {
		input.Paths = dangerToolPaths(payload)
		input.Writes = isMemoryWriteTool(payload.ToolName)
	}
	return guard.Decide(input, hookPolicy())
}

func dangerToolPaths(payload hookEvent) []string {
	paths := memoryToolPaths(payload)
	var generic map[string]any
	if err := json.Unmarshal(payload.ToolInput, &generic); err != nil {
		return paths
	}
	keys := []string{"path", "glob"}
	if payload.ToolName == "Glob" {
		keys = append(keys, "pattern")
	}
	for _, key := range keys {
		if text, ok := generic[key].(string); ok && text != "" {
			paths = append(paths, text)
		}
	}
	return paths
}

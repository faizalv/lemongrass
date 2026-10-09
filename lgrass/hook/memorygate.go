package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/guard"
	"github.com/faizalv/lemongrass/session"
)

const memoryWriteWord = session.MemoryWriteWord

// Short enough to resurface again within the same session if memory writes are still happening.
const memoryFeedbackSignatureTTL = 30 * time.Minute

type notebookToolInput struct {
	NotebookPath string `json:"notebook_path"`
}

type shellToolInput struct {
	Command string `json:"command"`
}

var (
	claudeConfigDirName     = regexp.MustCompile(`^\.claude(-[a-z0-9-]+)?$`)
	claudeProjectsInCommand = regexp.MustCompile(`\.claude(-[a-z0-9-]+)?/projects`)
	shellDevNullRedirect    = regexp.MustCompile(`\d*>&?\s*/dev/null`)
	shellWriteIndicator     = regexp.MustCompile(`>|<<|\btee\b|\bsed\b[^|;&]*\s-[a-zA-Z]*i|--in-place|\b(cp|mv|install|dd|truncate|rm|ln|touch|patch|rsync|ed|ex|vi|vim|nano|emacs|curl|wget|python[0-9.]*|node|ruby|perl|php|deno|bun)\b|\b(sh|bash|zsh)\s+-c\b|\beval\b`)
)

// memoryFeedbackDeny gates every route that can write into a Claude Code memory directory (file tools, patches, notebooks, shell commands) behind a sign, once per session per memoryFeedbackSignatureTTL.
func memoryFeedbackDeny(store *session.Store, payload hookEvent, projectPath string) string {
	if !touchesClaudeMemory(payload, projectPath) {
		return ""
	}
	signedAt, err := store.SignedAt(payload.SessionID, memoryWriteWord)
	if err == nil && !signedAt.IsZero() && time.Since(signedAt) <= memoryFeedbackSignatureTTL {
		return ""
	}
	return session.FormatMemoryFeedbackDeny()
}

func touchesClaudeMemory(payload hookEvent, projectPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	projectMemDir, _ := claudeMemoryDir(projectPath)

	if payload.ToolName == "Bash" {
		return shellTouchesClaudeMemory(payload, home, projectMemDir)
	}
	if !isMemoryWriteTool(payload.ToolName) {
		return false
	}
	for _, p := range memoryToolPaths(payload) {
		if isClaudeMemoryPath(p, payload.Cwd, home, projectMemDir) {
			return true
		}
	}
	return false
}

func isMemoryWriteTool(name string) bool {
	switch name {
	case "Write", "Edit", "MultiEdit", "NotebookEdit", "apply_patch":
		return true
	}
	return false
}

func memoryToolPaths(payload hookEvent) []string {
	paths := toolFilePaths(payload)
	var nb notebookToolInput
	if err := json.Unmarshal(payload.ToolInput, &nb); err == nil && nb.NotebookPath != "" {
		paths = append(paths, nb.NotebookPath)
	}
	return paths
}

// isClaudeMemoryPath matches any project's memory directory under the projects directory of ~/.claude or of any ~/.claude-<account>, not only this project's, so a slug-algorithm mismatch or a sibling project's directory does not open a gap.
func isClaudeMemoryPath(p, cwd, home, projectMemDir string) bool {
	p = guard.ResolvePath(p, cwd, home)
	if projectMemDir != "" && strings.HasPrefix(p+string(filepath.Separator), projectMemDir) {
		return true
	}
	prefix := home + string(filepath.Separator)
	if !strings.HasPrefix(p, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, prefix), string(filepath.Separator))
	return len(parts) >= 4 && claudeConfigDirName.MatchString(parts[0]) && parts[1] == "projects" && parts[3] == "memory"
}

// A shell command counts when it names a memory directory, or runs from inside one, and contains something that can write. Plain reads (cat, ls, grep, head) pass.
func shellTouchesClaudeMemory(payload hookEvent, home, projectMemDir string) bool {
	var in shellToolInput
	if err := json.Unmarshal(payload.ToolInput, &in); err != nil || in.Command == "" {
		return false
	}
	if !shellWriteIndicator.MatchString(shellDevNullRedirect.ReplaceAllString(in.Command, "")) {
		return false
	}
	if payload.Cwd != "" && isClaudeMemoryPath(filepath.Join(payload.Cwd, "x"), "", home, projectMemDir) {
		return true
	}
	cmd := strings.ReplaceAll(in.Command, "\\", "")
	return claudeProjectsInCommand.MatchString(cmd) && strings.Contains(cmd, "memory") ||
		projectMemDir != "" && strings.Contains(cmd, strings.TrimSuffix(projectMemDir, string(filepath.Separator)))
}

// claudeMemoryDir mirrors Claude Code's own project-slug algorithm: the resolved absolute project path with every path separator replaced by "-". projectPath must already be symlink-resolved, which project.Resolve guarantees.
func claudeMemoryDir(projectPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(home, ".claude")
	}
	slug := strings.ReplaceAll(projectPath, string(filepath.Separator), "-")
	return filepath.Join(configDir, "projects", slug, "memory") + string(filepath.Separator), nil
}

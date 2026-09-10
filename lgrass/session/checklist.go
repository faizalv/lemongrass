package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultChecklistTTL = 5 * time.Minute

// Checklist gates a matching tool call behind an explicit `lgrass sign` until its signature's TTL expires.
type Checklist struct {
	ID           string `json:"id"`
	Tool         string `json:"tool"`          // pipe-separated tool names, "" matches any
	PathContains string `json:"path_contains"` // substring match against the tool's file path, "" matches any
	Content      string `json:"content"`       // shown to the model in the deny message
	TTLMinutes   int    `json:"ttl_minutes"`   // 0 falls back to defaultChecklistTTL
}

func (c Checklist) TTL() time.Duration {
	if c.TTLMinutes <= 0 {
		return defaultChecklistTTL
	}
	return time.Duration(c.TTLMinutes) * time.Minute
}

func (c Checklist) Matches(toolName, filePath string) bool {
	if !matchesTool(c.Tool, toolName) {
		return false
	}
	if c.PathContains != "" && !strings.Contains(filePath, c.PathContains) {
		return false
	}
	return true
}

func matchesTool(pattern, toolName string) bool {
	if pattern == "" {
		return true
	}
	for _, p := range strings.Split(pattern, "|") {
		if p == toolName {
			return true
		}
	}
	return false
}

// LoadChecklists reads .lgrass/checklists.json from the project root. A missing file means no checklists configured, not an error.
func LoadChecklists(projectPath string) ([]Checklist, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, ".lgrass", "checklists.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Checklist
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

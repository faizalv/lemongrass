package usecase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/modules/lg/entity"
	"github.com/pmezard/go-difflib/difflib"
)

func (u *LgUsecase) computeExecDiff(workspaceID string) {
	u.mu.Lock()
	snapshots := u.beforeSnapshots[workspaceID]
	var paths []string
	seen := make(map[string]bool)
	for _, e := range u.writeTrail {
		if e.SessionID == workspaceID && !seen[e.FilePath] {
			paths = append(paths, e.FilePath)
			seen[e.FilePath] = true
		}
	}
	delete(u.beforeSnapshots, workspaceID)
	u.mu.Unlock()

	if len(paths) == 0 {
		return
	}

	var diffs []entity.FileDiff
	for _, path := range paths {
		before := snapshots[path]
		afterBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		diffs = append(diffs, buildDiff(before, string(afterBytes), path))
	}

	u.mu.Lock()
	u.execDiffs[workspaceID] = diffs
	u.mu.Unlock()

	diffPath := execDiffPath(workspaceID)
	if data, err := json.Marshal(diffs); err == nil {
		os.WriteFile(diffPath, data, 0644)
	}
}

func (u *LgUsecase) GetExecutionDiff(workspaceID string) []entity.FileDiff {
	u.mu.Lock()
	cached, ok := u.execDiffs[workspaceID]
	u.mu.Unlock()
	if ok {
		return cached
	}
	data, err := os.ReadFile(execDiffPath(workspaceID))
	if err != nil {
		return nil
	}
	var result []entity.FileDiff
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func execDiffPath(workspaceID string) string {
	return filepath.Join(config.Dir(), "workspaces", workspaceID, "execution-diff.json")
}

func buildDiff(before, after, filePath string) entity.FileDiff {
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: filePath,
		ToFile:   filePath,
		Context:  3,
	}
	text, _ := difflib.GetUnifiedDiffString(ud)

	added, removed := 0, 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	return entity.FileDiff{
		FilePath:     filePath,
		Diff:         text,
		IsNew:        before == "",
		LinesAdded:   added,
		LinesRemoved: removed,
	}
}

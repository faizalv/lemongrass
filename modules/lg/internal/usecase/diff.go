package usecase

import (
	"os"
	"strings"

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
}

func (u *LgUsecase) GetExecutionDiff(workspaceID string) []entity.FileDiff {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.execDiffs[workspaceID]
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

package usecase

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

func (u *ReconUsecase) GitStatus(ctx context.Context, projectID int64) (entity.GitStatus, error) {
	rawPath, err := u.repo.ProjectDir(ctx, projectID)
	if err != nil {
		return entity.GitStatus{}, err
	}
	dir := "/projects/" + filepath.Base(rawPath)

	staleCount, _ := u.repo.GetStaleCount(ctx, projectID)

	head, err := gitCmd(dir, "rev-parse", "HEAD")
	if err != nil {
		return entity.GitStatus{IsGitRepo: false, StaleCount: staleCount}, nil
	}
	head = strings.TrimSpace(head)
	short := head
	if len(short) > 7 {
		short = short[:7]
	}

	branch, _ := gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)

	headMsg, _ := gitCmd(dir, "log", "-1", "--pretty=%s")
	headMsg = strings.TrimSpace(headMsg)

	pathStatus := make(map[string]string)
	collectGitStatus(pathStatus, dir, "diff", "--name-status")
	collectGitStatus(pathStatus, dir, "diff", "--name-status", "--cached")
	lastCommit, _ := u.repo.GetLastSyncedCommit(ctx, projectID)
	if lastCommit != "" && lastCommit != head {
		collectGitStatus(pathStatus, dir, "diff", "--name-status", lastCommit, head)
	}

	changed := make([]entity.ChangedFile, 0, len(pathStatus))
	for p, s := range pathStatus {
		changed = append(changed, entity.ChangedFile{Path: p, Status: s})
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Path < changed[j].Path })

	var commits []entity.CommitInfo
	if out, err := gitCmd(dir, "log", "--pretty=%h\t%s\t%an\t%aI", "-10"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) < 4 {
				continue
			}
			commits = append(commits, entity.CommitInfo{
				Hash:      parts[0],
				Message:   parts[1],
				Author:    parts[2],
				Timestamp: parts[3],
			})
		}
	}

	return entity.GitStatus{
		IsGitRepo:     true,
		Branch:        branch,
		HeadCommit:    short,
		HeadMessage:   headMsg,
		ChangedFiles:  changed,
		StaleCount:    staleCount,
		RecentCommits: commits,
	}, nil
}

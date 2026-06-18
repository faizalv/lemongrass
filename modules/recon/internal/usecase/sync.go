package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

// Sync detects file-level changes and re-maps the project if anything changed.
// On first run (no stored hashes) it always calls Map and writes the baseline.
func (u *ReconUsecase) Sync(ctx context.Context, projectID int64, dir string) error {
	ig := loadIgnore(dir)

	var parsedCandidates []string
	var ignoredExisting []string

	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if ig.Match(rel) {
			ignoredExisting = append(ignoredExisting, rel)
		} else {
			parsedCandidates = append(parsedCandidates, rel)
		}
		return nil
	}); err != nil {
		return err
	}

	currentHashes := make(map[string]string, len(parsedCandidates))
	for _, rel := range parsedCandidates {
		h, err := hashFile(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		currentHashes[rel] = h
	}

	storedHashes, err := u.repo.GetFileHashes(ctx, projectID)
	if err != nil {
		return err
	}

	var toUpsert []entity.FileHash
	var toDelete []string
	changed := len(storedHashes) == 0

	for path, hash := range currentHashes {
		if stored, exists := storedHashes[path]; !exists || stored != hash {
			toUpsert = append(toUpsert, entity.FileHash{Path: path, Hash: hash})
			changed = true
		}
	}
	for path := range storedHashes {
		if _, exists := currentHashes[path]; !exists {
			toDelete = append(toDelete, path)
			changed = true
		}
	}

	u.pathCacheMu.Lock()
	u.pathCache[projectID] = parsedCandidates
	u.pathCacheMu.Unlock()

	if !changed {
		return nil
	}

	if err := u.Map(ctx, projectID, dir, ignoredExisting); err != nil {
		return err
	}

	if len(toUpsert) > 0 {
		if err := u.repo.UpsertFileHashes(ctx, projectID, toUpsert); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		if err := u.repo.DeleteFileHashes(ctx, projectID, toDelete); err != nil {
			return err
		}
	}

	return nil
}

// MapFiles re-parses only the listed file paths and upserts the resulting nodes.
// Nodes for paths that produce no output (deleted files) are removed.
func (u *ReconUsecase) MapFiles(ctx context.Context, projectID int64, dir string, paths []string, branch string) error {
	ig := loadIgnore(dir)
	var filtered []string
	for _, p := range paths {
		if !ig.Match(p) {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	var allNodes []entity.SemanticNode
	for _, p := range u.parsers {
		if !p.Detect(dir) {
			continue
		}
		result, err := p.ParseFiles(dir, ig, filtered)
		if err != nil {
			log.Printf("[recon] parser %s: %v", p.Name(), err)
			continue
		}
		allNodes = append(allNodes, u.NodesToInsert(projectID, []*entity.ParseResult{result})...)
	}

	if len(allNodes) > 0 {
		if err := u.repo.UpsertNodes(ctx, allNodes, branch); err != nil {
			return err
		}
	}

	if branch != "" {
		existing, err := u.repo.ListNodesInFilesWithBranch(ctx, projectID, filtered, branch)
		if err != nil {
			return err
		}
		alive := make(map[string]bool, len(allNodes))
		for _, n := range allNodes {
			alive[n.FilePath+"\x00"+n.Symbol+"\x00"+n.Kind+"\x00"+n.ContentHash] = true
		}
		for _, n := range existing {
			if !alive[n.FilePath+"\x00"+n.Symbol+"\x00"+n.Kind+"\x00"+n.ContentHash] {
				_ = u.repo.RemoveBranchFromNode(ctx, projectID, n.FilePath, n.Symbol, n.Kind, n.ContentHash, branch)
			}
		}
	}

	producedPaths := make(map[string]bool, len(allNodes))
	for _, n := range allNodes {
		producedPaths[n.FilePath] = true
	}
	var gone []string
	for _, p := range filtered {
		if !producedPaths[p] {
			gone = append(gone, p)
		}
	}
	if len(gone) > 0 {
		return u.repo.DeleteNodesByFilePaths(ctx, projectID, gone)
	}
	return nil
}

// SyncGit uses git diff to detect changed files and re-maps only those.
// Falls back to full Sync if the directory is not a git repository.
// On branch switch, applies a two-tier strategy:
//   - same HEAD tip: bulk-stamp all nodes with the new branch, no rescan
//   - different HEAD tips: bulk-stamp unchanged files + rescan only the delta
func (u *ReconUsecase) SyncGit(ctx context.Context, projectID int64, dir string) error {
	head, err := gitCmd(dir, "rev-parse", "HEAD")
	if err != nil {
		return u.Sync(ctx, projectID, dir)
	}
	head = strings.TrimSpace(head)

	currentBranch := resolveBranch(dir)
	lastCommit, _ := u.repo.GetLastSyncedCommit(ctx, projectID)
	lastBranch, _ := u.repo.GetLastSyncedBranch(ctx, projectID)

	if lastBranch != "" && currentBranch != lastBranch {
		if lastCommit != "" && lastCommit == head {
			if err := u.repo.BulkStampBranch(ctx, projectID, lastBranch, currentBranch); err != nil {
				return err
			}
		} else if lastCommit != "" {
			changedSet := make(map[string]bool)
			collectGitPaths(changedSet, dir, "diff", "--name-only", lastCommit, head)
			changed := make([]string, 0, len(changedSet))
			for p := range changedSet {
				changed = append(changed, p)
			}
			if err := u.repo.BulkStampBranchForFiles(ctx, projectID, lastBranch, currentBranch, changed); err != nil {
				return err
			}
			if len(changed) > 0 {
				if err := u.MapFiles(ctx, projectID, dir, changed, currentBranch); err != nil {
					return err
				}
			}
		} else {
			if err := u.Map(ctx, projectID, dir, nil); err != nil {
				return err
			}
		}
		_ = u.repo.SetLastSyncedBranch(ctx, projectID, currentBranch)
		_ = u.repo.SetLastSyncedCommit(ctx, projectID, head)
		return nil
	}

	pathSet := make(map[string]bool)
	collectGitPaths(pathSet, dir, "diff", "--name-only")
	collectGitPaths(pathSet, dir, "diff", "--name-only", "--cached")
	if lastCommit != "" && lastCommit != head {
		collectGitPaths(pathSet, dir, "diff", "--name-only", lastCommit, head)
	}

	if err := u.repo.SetLastSyncedCommit(ctx, projectID, head); err != nil {
		return err
	}
	if lastBranch == "" {
		_ = u.repo.SetLastSyncedBranch(ctx, projectID, currentBranch)
	}

	if len(pathSet) == 0 {
		return nil
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	if err := u.MapFiles(ctx, projectID, dir, paths, currentBranch); err != nil {
		return err
	}
	if cached, err := u.repo.ListFilePaths(ctx, projectID); err == nil {
		u.pathCacheMu.Lock()
		u.pathCache[projectID] = cached
		u.pathCacheMu.Unlock()
	}
	return nil
}

// ActivateGitSync triggers a non-blocking SyncGit for the given project.
// No-op if a sync is already running.
func (u *ReconUsecase) ActivateGitSync(projectID int64) {
	u.syncMu.Lock()
	if u.syncing[projectID] {
		u.syncMu.Unlock()
		return
	}
	u.syncing[projectID] = true
	u.syncMu.Unlock()

	go func() {
		defer func() {
			u.syncMu.Lock()
			u.syncing[projectID] = false
			u.lastSynced[projectID] = time.Now().UnixNano()
			u.syncMu.Unlock()
		}()
		rawPath, err := u.repo.ProjectDir(context.Background(), projectID)
		if err != nil {
			return
		}
		dir := "/projects/" + filepath.Base(rawPath)
		u.SyncGit(context.Background(), projectID, dir)
	}()
}

// TickGitPoller is called every 5s by the scheduler for git-aware projects.
func (u *ReconUsecase) TickGitPoller(ctx context.Context) {
	u.syncMu.Lock()
	activeID := u.activeID
	u.syncMu.Unlock()
	if activeID == 0 {
		return
	}
	u.ActivateGitSync(activeID)
}

// resolveBranch returns the current git branch name.
// Returns "HEAD" for detached HEAD state, "init" if git is unavailable.
func resolveBranch(dir string) string {
	out, err := gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "init"
	}
	b := strings.TrimSpace(out)
	if b == "" {
		return "init"
	}
	return b
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

func collectGitPaths(set map[string]bool, dir string, args ...string) {
	out, err := gitCmd(dir, args...)
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			set[line] = true
		}
	}
}

func collectGitStatus(m map[string]string, dir string, args ...string) {
	out, err := gitCmd(dir, args...)
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if _, exists := m[parts[1]]; !exists {
			m[parts[1]] = gitStatusLabel(parts[0])
		}
	}
}

func gitStatusLabel(code string) string {
	switch code {
	case "A":
		return "added"
	case "D":
		return "deleted"
	default:
		return "modified"
	}
}

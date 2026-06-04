package usecase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

func (u *ReconUsecase) ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error) {
	return u.repo.ListNodes(ctx, projectID, language, kind, status)
}

func (u *ReconUsecase) GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error) {
	return u.repo.GetCoverage(ctx, projectID)
}

func (u *ReconUsecase) TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]entity.DirectoryCoverage, error) {
	return u.repo.GetTreeCoverage(ctx, projectID, pathPrefix)
}

func (u *ReconUsecase) ReadNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (entity.SemanticNode, string, error) {
	node, err := u.repo.GetNode(ctx, projectID, filePath, symbol, kind)
	if err != nil {
		return entity.SemanticNode{}, "", fmt.Errorf("node not found: %w", err)
	}
	rawPath, err := u.repo.ProjectDir(ctx, projectID)
	if err != nil {
		return entity.SemanticNode{}, "", err
	}
	diskPath := filepath.Join("/projects", filepath.Base(rawPath), filePath)
	code, err := readLines(diskPath, node.LineStart, node.LineEnd)
	if err != nil {
		return entity.SemanticNode{}, "", fmt.Errorf("read file: %w", err)
	}
	return node, code, nil
}

func (u *ReconUsecase) Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) error {
	if err := u.repo.AnnotateNode(ctx, projectID, filePath, symbol, kind, description, returnType, calls); err != nil {
		return err
	}
	go func() {
		vec, err := u.embed.Embed(context.Background(), description)
		if err != nil {
			return
		}
		u.repo.SetEmbedding(context.Background(), projectID, filePath, symbol, vec)
	}()
	return nil
}

func (u *ReconUsecase) GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error) {
	return u.repo.GetProjectCoverage(ctx, projectID)
}

func (u *ReconUsecase) DropFile(ctx context.Context, projectID int64, path string) {
	u.repo.DeleteNodesByFilePaths(ctx, projectID, []string{path})
	u.repo.DeleteFileHashes(ctx, projectID, []string{path})
}

func (u *ReconUsecase) Search(ctx context.Context, projectID int64, query string) ([]entity.SemanticNode, error) {
	const limit = 10
	var results []entity.SemanticNode

	vec, err := u.embed.Embed(ctx, query)
	if err == nil {
		results, err = u.repo.SearchByVector(ctx, projectID, vec, limit)
		if err != nil {
			results = nil
		}
	}

	fts, err := u.repo.SearchByFTS(ctx, projectID, query, limit)
	if err == nil {
		seen := make(map[string]bool, len(results))
		for _, n := range results {
			seen[n.ID] = true
		}
		for _, n := range fts {
			if !seen[n.ID] {
				results = append(results, n)
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (u *ReconUsecase) Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []entity.SemanticNode, err error) {
	return u.repo.GetRelated(ctx, projectID, filePath, symbol, kind)
}

func (u *ReconUsecase) PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, error) {
	return u.repo.ListByPathPrefix(ctx, projectID, pathPrefix)
}

func readLines(path string, start, end int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if line >= start {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(scanner.Text())
		}
		if line >= end {
			break
		}
	}
	return sb.String(), scanner.Err()
}

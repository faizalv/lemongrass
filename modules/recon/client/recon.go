package client

import (
	"context"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
)

type ReconClient struct {
	uc *usecase.ReconUsecase
}

func New(uc *usecase.ReconUsecase) *ReconClient {
	return &ReconClient{uc: uc}
}

func (c *ReconClient) TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]entity.DirectoryCoverage, error) {
	return c.uc.TreeCoverage(ctx, projectID, pathPrefix)
}

func (c *ReconClient) ReadNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (entity.SemanticNode, string, error) {
	return c.uc.ReadNode(ctx, projectID, filePath, symbol, kind)
}

func (c *ReconClient) Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) (int64, error) {
	return c.uc.Annotate(ctx, projectID, filePath, symbol, kind, description, returnType, calls)
}

func (c *ReconClient) GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error) {
	return c.uc.GetProjectCoverage(ctx, projectID)
}

func (c *ReconClient) Search(ctx context.Context, projectID int64, query string) ([]entity.SemanticNode, error) {
	return c.uc.Search(ctx, projectID, query)
}

func (c *ReconClient) Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []entity.SemanticNode, err error) {
	return c.uc.Related(ctx, projectID, filePath, symbol, kind)
}

func (c *ReconClient) PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, []entity.SubdirSummary, error) {
	return c.uc.PeekDir(ctx, projectID, pathPrefix)
}

func (c *ReconClient) ListAllNodesByPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, error) {
	return c.uc.ListAllNodesByPrefix(ctx, projectID, pathPrefix)
}

func (c *ReconClient) DropFile(ctx context.Context, projectID int64, path string) {
	c.uc.DropFile(ctx, projectID, path)
}

func (c *ReconClient) SyncGitProject(projectID int64) {
	c.uc.ActivateGitSync(projectID)
}

func (c *ReconClient) SaveKnowledge(ctx context.Context, projectID int64, key, content string, labels []string) (bool, error) {
	return c.uc.SaveKnowledge(ctx, projectID, key, content, labels)
}

func (c *ReconClient) ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error) {
	return c.uc.ReadKnowledge(ctx, projectID, key)
}

func (c *ReconClient) SearchKnowledge(ctx context.Context, projectID int64, query, label string) ([]entity.KnowledgeEntry, bool, error) {
	return c.uc.SearchKnowledge(ctx, projectID, query, label)
}

func (c *ReconClient) DeleteKnowledge(ctx context.Context, projectID int64, key string) (bool, error) {
	return c.uc.DeleteKnowledge(ctx, projectID, key)
}

func (c *ReconClient) FindSimilarKnowledge(ctx context.Context, projectID int64, content, excludeKey string) ([]string, error) {
	return c.uc.FindSimilarKnowledge(ctx, projectID, content, excludeKey)
}

func (c *ReconClient) UpsertLabel(ctx context.Context, projectID int64, label, content string) error {
	return c.uc.UpsertLabel(ctx, projectID, label, content)
}

func (c *ReconClient) FindSimilarLabels(ctx context.Context, projectID int64, label, content string) ([]string, error) {
	return c.uc.FindSimilarLabels(ctx, projectID, label, content)
}

func (c *ReconClient) ListAllLabels(ctx context.Context, projectID int64) ([]string, error) {
	return c.uc.ListAllLabels(ctx, projectID)
}

func (c *ReconClient) SearchLabels(ctx context.Context, projectID int64, query string) ([]string, error) {
	return c.uc.SearchLabels(ctx, projectID, query)
}

func (c *ReconClient) SearchKnowledgeByLabel(ctx context.Context, projectID int64, label, query string) ([]entity.KnowledgeEntry, error) {
	return c.uc.SearchKnowledgeByLabel(ctx, projectID, label, query)
}

func (c *ReconClient) ListFileNodes(ctx context.Context, projectID int64, filePath string) ([]entity.SemanticNode, error) {
	return c.uc.ListFileNodes(ctx, projectID, filePath)
}

func (c *ReconClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.uc.Embed(ctx, text)
}

func (c *ReconClient) ProjectDir(ctx context.Context, projectID int64) (string, error) {
	return c.uc.ProjectDir(ctx, projectID)
}

func (c *ReconClient) ListFilePaths(ctx context.Context, projectID int64) []string {
	return c.uc.ListFilePaths(ctx, projectID)
}

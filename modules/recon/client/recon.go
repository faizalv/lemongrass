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

func (c *ReconClient) DropFile(ctx context.Context, projectID int64, path string) {
	c.uc.DropFile(ctx, projectID, path)
}

func (c *ReconClient) SyncGitProject(projectID int64) {
	c.uc.ActivateGitSync(projectID)
}

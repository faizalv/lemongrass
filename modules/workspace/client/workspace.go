package client

import (
	"context"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
	"github.com/faizalv/lemongrass/modules/workspace/internal/repository"
)

type WorkspaceTaskClient struct {
	repo *repository.WorkspaceRepository
}

func New(repo *repository.WorkspaceRepository) *WorkspaceTaskClient {
	return &WorkspaceTaskClient{repo: repo}
}

func (c *WorkspaceTaskClient) CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error) {
	return c.repo.CreateTasks(ctx, workspaceID, tasks)
}

func (c *WorkspaceTaskClient) UpdateStatus(ctx context.Context, id, status string) error {
	return c.repo.UpdateStatus(ctx, id, status)
}

func (c *WorkspaceTaskClient) GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	return c.repo.GetTasks(ctx, workspaceID)
}

func (c *WorkspaceTaskClient) SaveHandoverContext(ctx context.Context, workspaceID, context string) error {
	return c.repo.SaveHandoverContext(ctx, workspaceID, context)
}

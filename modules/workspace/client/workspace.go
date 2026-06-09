package client

import (
	"context"
	"fmt"

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

func (c *WorkspaceTaskClient) CreateWorkspace(ctx context.Context, projectID int64, name string) (entity.Workspace, error) {
	return c.repo.Create(ctx, entity.Workspace{ProjectID: projectID, Name: name})
}

func (c *WorkspaceTaskClient) FindWorkspace(ctx context.Context, projectID int64, nameOrID string) (entity.Workspace, error) {
	workspaces, err := c.repo.ListByProject(ctx, projectID, false)
	if err != nil {
		return entity.Workspace{}, err
	}
	for _, ws := range workspaces {
		if ws.ID == nameOrID || ws.Name == nameOrID {
			return ws, nil
		}
	}
	return entity.Workspace{}, fmt.Errorf("workspace not found: %s", nameOrID)
}

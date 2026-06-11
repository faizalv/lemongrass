package client

import (
	"context"
	"fmt"
	"time"

	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
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

func (c *WorkspaceTaskClient) AddTextRequirement(ctx context.Context, workspaceID, text string) error {
	_, err := c.repo.AddTextRequirement(ctx, workspaceID, text)
	return err
}

func (c *WorkspaceTaskClient) ListWorkspaces(ctx context.Context, projectID int64) ([]entity.Workspace, error) {
	return c.repo.ListByProject(ctx, projectID, false)
}

func (c *WorkspaceTaskClient) GetWorkspace(ctx context.Context, id string) (entity.Workspace, error) {
	return c.repo.Get(ctx, id)
}

func (c *WorkspaceTaskClient) DeleteWorkspace(ctx context.Context, id string) error {
	ws, err := c.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace must be idle to delete (current status: %s)", ws.Status)
	}
	return c.repo.UpdateStatus(ctx, id, "deleted")
}

func (c *WorkspaceTaskClient) GetTask(ctx context.Context, taskID string) (entity.Task, error) {
	return c.repo.GetTask(ctx, taskID)
}

func (c *WorkspaceTaskClient) GetTaskByNumber(ctx context.Context, workspaceID string, num int) (entity.Task, error) {
	return c.repo.GetTaskByNumber(ctx, workspaceID, num)
}

func (c *WorkspaceTaskClient) StartTask(ctx context.Context, taskID string, startedAt time.Time) error {
	return c.repo.StartTask(ctx, taskID, startedAt)
}

func (c *WorkspaceTaskClient) FinishTask(ctx context.Context, taskID, notes string, diff []lgentity.FileDiff, finishedAt time.Time) error {
	return c.repo.FinishTask(ctx, taskID, notes, diff, finishedAt)
}

func (c *WorkspaceTaskClient) RejectTask(ctx context.Context, taskID, reason string) error {
	return c.repo.RejectTask(ctx, taskID, reason)
}

func (c *WorkspaceTaskClient) GetRejectedTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	return c.repo.GetRejectedTasks(ctx, workspaceID)
}

func (c *WorkspaceTaskClient) CountInProgressTasks(ctx context.Context, workspaceID string) (int, error) {
	return c.repo.CountInProgressTasks(ctx, workspaceID)
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

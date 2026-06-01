package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type workspaceStore interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error)
	CountExecuting(ctx context.Context, projectID int64) (int, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetProjectPath(ctx context.Context, projectID int64) (string, error)
}

type WorkspaceUsecase struct {
	ws workspaceStore
}

func NewWorkspace(ws workspaceStore) *WorkspaceUsecase {
	return &WorkspaceUsecase{ws: ws}
}

func (u *WorkspaceUsecase) Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error) {
	return u.ws.Create(ctx, ws)
}

func (u *WorkspaceUsecase) Get(ctx context.Context, id string) (entity.Workspace, error) {
	return u.ws.Get(ctx, id)
}

func (u *WorkspaceUsecase) ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error) {
	return u.ws.ListByProject(ctx, projectID, includeDeleted)
}

func (u *WorkspaceUsecase) DeleteWorkspace(ctx context.Context, id string) error {
	ws, err := u.ws.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("workspace not found")
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace must be idle to delete")
	}
	return u.ws.UpdateStatus(ctx, id, "deleted")
}

func (u *WorkspaceUsecase) IsExecutionLocked(ctx context.Context, projectID int64) (bool, error) {
	n, err := u.ws.CountExecuting(ctx, projectID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (u *WorkspaceUsecase) UpdateStatus(ctx context.Context, id, status string) error {
	return u.ws.UpdateStatus(ctx, id, status)
}

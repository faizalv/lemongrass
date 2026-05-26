package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type repo interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64) ([]entity.Workspace, error)
	UpdateRequirement(ctx context.Context, id, text, file, reqType string) error
	CountExecuting(ctx context.Context, projectID int64) (int, error)
}

type WorkspaceUsecase struct {
	repo repo
}

func New(r repo) *WorkspaceUsecase {
	return &WorkspaceUsecase{repo: r}
}

func (u *WorkspaceUsecase) Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error) {
	return u.repo.Create(ctx, ws)
}

func (u *WorkspaceUsecase) Get(ctx context.Context, id string) (entity.Workspace, error) {
	return u.repo.Get(ctx, id)
}

func (u *WorkspaceUsecase) ListByProject(ctx context.Context, projectID int64) ([]entity.Workspace, error) {
	return u.repo.ListByProject(ctx, projectID)
}

func (u *WorkspaceUsecase) ReplaceRequirement(ctx context.Context, id, text, file, reqType string) error {
	ws, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if ws.Status == "grooming" || ws.Status == "executing" {
		return fmt.Errorf("cannot replace requirement while workspace is %s", ws.Status)
	}
	return u.repo.UpdateRequirement(ctx, id, text, file, reqType)
}

func (u *WorkspaceUsecase) IsExecutionLocked(ctx context.Context, projectID int64) (bool, error) {
	n, err := u.repo.CountExecuting(ctx, projectID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

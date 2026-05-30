package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

func (u *WorkspaceUsecase) ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error) {
	return u.repo.ListRequirements(ctx, workspaceID)
}

func (u *WorkspaceUsecase) AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error) {
	ws, err := u.repo.Get(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	if ws.Status != "idle" {
		return entity.WorkspaceRequirement{}, fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.repo.AddTextRequirement(ctx, workspaceID, text)
}

func (u *WorkspaceUsecase) AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error) {
	ws, err := u.repo.Get(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	if ws.Status != "idle" {
		return entity.WorkspaceRequirement{}, fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.repo.AddFileRequirement(ctx, workspaceID, reqType, filePath, fileName)
}

func (u *WorkspaceUsecase) DeleteRequirement(ctx context.Context, workspaceID, reqID string) error {
	ws, err := u.repo.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.Status != "idle" {
		return fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.repo.DeleteRequirement(ctx, reqID)
}

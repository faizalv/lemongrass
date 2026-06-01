package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type requirementStore interface {
	ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error)
	AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error)
	AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error)
	DeleteRequirement(ctx context.Context, reqID string) error
	CountRequirements(ctx context.Context, workspaceID string) (int, error)
}

type RequirementUsecase struct {
	ws  workspaceStore
	req requirementStore
}

func NewRequirement(ws workspaceStore, req requirementStore) *RequirementUsecase {
	return &RequirementUsecase{ws: ws, req: req}
}

func (u *RequirementUsecase) ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error) {
	return u.req.ListRequirements(ctx, workspaceID)
}

func (u *RequirementUsecase) AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error) {
	ws, err := u.ws.Get(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	if ws.Status != "idle" {
		return entity.WorkspaceRequirement{}, fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.req.AddTextRequirement(ctx, workspaceID, text)
}

func (u *RequirementUsecase) AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error) {
	ws, err := u.ws.Get(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	if ws.Status != "idle" {
		return entity.WorkspaceRequirement{}, fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.req.AddFileRequirement(ctx, workspaceID, reqType, filePath, fileName)
}

func (u *RequirementUsecase) DeleteRequirement(ctx context.Context, workspaceID, reqID string) error {
	ws, err := u.ws.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.Status != "idle" {
		return fmt.Errorf("requirements locked while workspace is %s", ws.Status)
	}
	return u.req.DeleteRequirement(ctx, reqID)
}

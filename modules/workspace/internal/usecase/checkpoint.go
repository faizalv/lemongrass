package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

func (u *WorkspaceUsecase) GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	return u.repo.GetTasks(ctx, workspaceID)
}

func (u *WorkspaceUsecase) SaveTaskDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error {
	if u.draft == nil {
		return fmt.Errorf("draft store not configured")
	}
	return u.draft.SaveDecision(ctx, workspaceID, taskID, approved, feedback)
}

func (u *WorkspaceUsecase) GetCheckpointDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error) {
	if u.draft == nil {
		return map[string]entity.TaskDecision{}, nil
	}
	return u.draft.GetDraft(ctx, workspaceID)
}

func (u *WorkspaceUsecase) ApproveCheckpoint(ctx context.Context, workspaceID string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if err := u.repo.ApproveTasks(ctx, workspaceID); err != nil {
		return err
	}
	if u.draft != nil {
		u.draft.ClearDraft(ctx, workspaceID)
	}
	return u.lgSess.RespondToCheckpoint(workspaceID, nil)
}

func (u *WorkspaceUsecase) SubmitCheckpointReviews(ctx context.Context, workspaceID string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if u.draft == nil {
		return fmt.Errorf("draft store not configured")
	}
	tasks, err := u.repo.GetTasks(ctx, workspaceID)
	if err != nil {
		return err
	}
	draft, err := u.draft.GetDraft(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if _, ok := draft[t.ID]; !ok {
			return fmt.Errorf("task %q has no decision yet", t.Title)
		}
	}
	rejections := map[string]string{}
	for _, t := range tasks {
		d := draft[t.ID]
		if !d.Approved {
			rejections[t.ID] = d.Feedback
		}
	}
	u.draft.ClearDraft(ctx, workspaceID)
	if len(rejections) == 0 {
		if err := u.repo.ApproveTasks(ctx, workspaceID); err != nil {
			return err
		}
	}
	return u.lgSess.RespondToCheckpoint(workspaceID, rejections)
}

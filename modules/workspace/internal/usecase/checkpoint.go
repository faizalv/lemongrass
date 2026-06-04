package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type taskStore interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error)
	GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error)
	ApproveTasks(ctx context.Context, workspaceID string) error
	RejectTasks(ctx context.Context, rejections map[string]string) error
}

type draftStore interface {
	SaveDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error
	GetDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error)
	ClearDraft(ctx context.Context, workspaceID string) error
}

type checkpointSession interface {
	RespondToCheckpoint(workspaceID string, rejections map[string]string) error
}

type CheckpointUsecase struct {
	ws     workspaceStore
	tasks  taskStore
	draft  draftStore
	lgSess checkpointSession
}

func NewCheckpoint(ws workspaceStore, tasks taskStore, draft draftStore) *CheckpointUsecase {
	return &CheckpointUsecase{ws: ws, tasks: tasks, draft: draft}
}

func (u *CheckpointUsecase) SetLgSession(s checkpointSession) {
	u.lgSess = s
}

func (u *CheckpointUsecase) GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	return u.tasks.GetTasks(ctx, workspaceID)
}

func (u *CheckpointUsecase) SaveTaskDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error {
	if u.draft == nil {
		return fmt.Errorf("draft store not configured")
	}
	return u.draft.SaveDecision(ctx, workspaceID, taskID, approved, feedback)
}

func (u *CheckpointUsecase) GetCheckpointDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error) {
	if u.draft == nil {
		return map[string]entity.TaskDecision{}, nil
	}
	return u.draft.GetDraft(ctx, workspaceID)
}

func (u *CheckpointUsecase) ApproveCheckpoint(ctx context.Context, workspaceID string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if err := u.tasks.ApproveTasks(ctx, workspaceID); err != nil {
		return err
	}
	if err := u.ws.UpdateStatus(ctx, workspaceID, "awaiting_execution"); err != nil {
		return err
	}
	if u.draft != nil {
		u.draft.ClearDraft(ctx, workspaceID)
	}
	return u.lgSess.RespondToCheckpoint(workspaceID, nil)
}

func (u *CheckpointUsecase) SubmitCheckpointReviews(ctx context.Context, workspaceID string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if u.draft == nil {
		return fmt.Errorf("draft store not configured")
	}
	tasks, err := u.tasks.GetTasks(ctx, workspaceID)
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
		if err := u.tasks.ApproveTasks(ctx, workspaceID); err != nil {
			return err
		}
		if err := u.ws.UpdateStatus(ctx, workspaceID, "awaiting_execution"); err != nil {
			return err
		}
		return u.lgSess.RespondToCheckpoint(workspaceID, nil)
	}
	// Persist feedback so amendment session can recover if the live session is dead.
	if err := u.tasks.RejectTasks(ctx, rejections); err != nil {
		return err
	}
	// Send to live session if available; ignore error if session is dead --
	// the user can resume via StartAmendmentSession.
	if u.lgSess != nil {
		u.lgSess.RespondToCheckpoint(workspaceID, rejections) //nolint:errcheck
	}
	return nil
}

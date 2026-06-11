package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type executionStore interface {
	Get(ctx context.Context, id string) (entity.Workspace, error)
	CountExecuting(ctx context.Context, projectID int64) (int, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetProjectPath(ctx context.Context, projectID int64) (string, error)
	GetTask(ctx context.Context, taskID string) (entity.Task, error)
	RejectTask(ctx context.Context, taskID, reason string) error
	CountInProgressTasks(ctx context.Context, workspaceID string) (int, error)
}

type executionSession interface {
	RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session)
	ResetSession(workspaceID string)
	WriteToSession(workspaceID string, msg []byte) error
}

type ExecutionUsecase struct {
	store  executionStore
	pty    ptyProvider
	lgSess executionSession
}

func NewExecution(store executionStore, pty ptyProvider) *ExecutionUsecase {
	return &ExecutionUsecase{store: store, pty: pty}
}

func (u *ExecutionUsecase) SetLgSession(s executionSession) {
	u.lgSess = s
}

func (u *ExecutionUsecase) StartExecution(ctx context.Context, workspaceID string) error {
	if u.pty == nil || u.lgSess == nil {
		return fmt.Errorf("execution not configured")
	}
	ws, err := u.store.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if ws.Status != "awaiting_execution" {
		return fmt.Errorf("workspace is %s, must be awaiting_execution to start", ws.Status)
	}
	n, err := u.store.CountExecuting(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("lock check: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("another workspace is already executing on this project")
	}
	projectPath, err := u.store.GetProjectPath(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	alias := filepath.Base(projectPath)
	if err := u.store.UpdateStatus(ctx, workspaceID, "executing"); err != nil {
		return err
	}
	session, err := u.pty.Open(buildExecutionPrompt(alias, ws.HandoverContext), workspaceID, "execution")
	if err != nil {
		u.store.UpdateStatus(ctx, workspaceID, "awaiting_execution")
		return fmt.Errorf("start execution PTY: %w", err)
	}
	u.lgSess.RegisterSession(workspaceID, alias, "execution", ws.ProjectID, session)
	session.Write([]byte("Begin execution.\r"))
	return nil
}

func (u *ExecutionUsecase) RejectTask(ctx context.Context, workspaceID, taskID, reason string) error {
	task, err := u.store.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	if task.ExecutionStatus != "done" {
		return fmt.Errorf("can only reject done tasks (current status: %s)", task.ExecutionStatus)
	}
	if err := u.store.RejectTask(ctx, taskID, reason); err != nil {
		return err
	}
	if u.lgSess == nil {
		return nil
	}
	ws, err := u.store.Get(ctx, workspaceID)
	if err != nil || ws.Status != "executing" {
		return nil
	}
	n, err := u.store.CountInProgressTasks(ctx, workspaceID)
	if err != nil || n > 0 {
		return nil
	}
	msg := fmt.Sprintf("[lemongrass] Task %q was rejected: %s. Re-do this task before calling #lg!.done.\r", task.Title, reason)
	_ = u.lgSess.WriteToSession(workspaceID, []byte(msg))
	return nil
}

func (u *ExecutionUsecase) ForceStopExecution(ctx context.Context, workspaceID string) error {
	if u.lgSess != nil {
		u.lgSess.ResetSession(workspaceID)
	}
	return u.store.UpdateStatus(ctx, workspaceID, "awaiting_execution")
}

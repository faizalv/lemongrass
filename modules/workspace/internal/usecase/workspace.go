package usecase

import (
	"context"
	"fmt"
	"time"

	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type repo interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error)
	CountExecuting(ctx context.Context, projectID int64) (int, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetProjectPath(ctx context.Context, projectID int64) (string, error)
	CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error)
	GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error)
	ApproveTasks(ctx context.Context, workspaceID string) error
	ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error)
	AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error)
	AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error)
	DeleteRequirement(ctx context.Context, reqID string) error
	CountRequirements(ctx context.Context, workspaceID string) (int, error)
}

type draftStore interface {
	SaveDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error
	GetDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error)
	ClearDraft(ctx context.Context, workspaceID string) error
}

type ptyProvider interface {
	Open(prompt, sessionID, sessionType string) (ptyclient.Session, error)
}

type lgSession interface {
	RegisterSession(workspaceID, projectAlias string, projectID int64, session ptyclient.Session)
	RespondToCheckpoint(workspaceID string, rejections map[string]string) error
	GetSessionActivity(workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool)
	ResetSession(workspaceID string)
}

type WorkspaceUsecase struct {
	repo   repo
	pty    ptyProvider
	lgSess lgSession
	draft  draftStore
}

func New(r repo) *WorkspaceUsecase {
	return &WorkspaceUsecase{repo: r}
}

func (u *WorkspaceUsecase) SetPty(p ptyProvider) {
	u.pty = p
}

func (u *WorkspaceUsecase) SetLgSession(s lgSession) {
	u.lgSess = s
}

func (u *WorkspaceUsecase) SetDraftStore(d draftStore) {
	u.draft = d
}

func (u *WorkspaceUsecase) Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error) {
	return u.repo.Create(ctx, ws)
}

func (u *WorkspaceUsecase) Get(ctx context.Context, id string) (entity.Workspace, error) {
	return u.repo.Get(ctx, id)
}

func (u *WorkspaceUsecase) ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error) {
	return u.repo.ListByProject(ctx, projectID, includeDeleted)
}

func (u *WorkspaceUsecase) DeleteWorkspace(ctx context.Context, id string) error {
	ws, err := u.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("workspace not found")
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace must be idle to delete")
	}
	return u.repo.UpdateStatus(ctx, id, "deleted")
}

func (u *WorkspaceUsecase) IsExecutionLocked(ctx context.Context, projectID int64) (bool, error) {
	n, err := u.repo.CountExecuting(ctx, projectID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (u *WorkspaceUsecase) UpdateStatus(ctx context.Context, id, status string) error {
	return u.repo.UpdateStatus(ctx, id, status)
}

func (u *WorkspaceUsecase) CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error) {
	return u.repo.CreateTasks(ctx, workspaceID, tasks)
}

func (u *WorkspaceUsecase) GetSessionActivity(ctx context.Context, workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool) {
	if u.lgSess == nil {
		return time.Time{}, -1, nil, false
	}
	return u.lgSess.GetSessionActivity(workspaceID)
}

func (u *WorkspaceUsecase) ResetSession(ctx context.Context, workspaceID string) error {
	if u.lgSess != nil {
		u.lgSess.ResetSession(workspaceID)
	}
	return u.repo.UpdateStatus(ctx, workspaceID, "idle")
}

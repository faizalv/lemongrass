package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type repo interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64) ([]entity.Workspace, error)
	UpdateRequirement(ctx context.Context, id, text, file, reqType string) error
	CountExecuting(ctx context.Context, projectID int64) (int, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetProjectPath(ctx context.Context, projectID int64) (string, error)
	CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error)
	GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error)
	ApproveTasks(ctx context.Context, workspaceID string) error
	DeletePendingTasks(ctx context.Context, workspaceID string) error
}

type WorkspaceUsecase struct {
	repo   repo
	pty    *ptyclient.PtyClient
	lgSess *lgclient.SessionManager
}

func New(r repo) *WorkspaceUsecase {
	return &WorkspaceUsecase{repo: r}
}

func (u *WorkspaceUsecase) SetPty(p *ptyclient.PtyClient) {
	u.pty = p
}

func (u *WorkspaceUsecase) SetLgSession(s *lgclient.SessionManager) {
	u.lgSess = s
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

func (u *WorkspaceUsecase) StartGrooming(ctx context.Context, workspaceID string) error {
	if u.pty == nil || u.lgSess == nil {
		return fmt.Errorf("grooming not configured")
	}
	ws, err := u.repo.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace is %s, must be idle to start grooming", ws.Status)
	}
	projectPath, err := u.repo.GetProjectPath(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	prompt := buildGroomingPrompt(ws, projectPath)
	if err := u.repo.UpdateStatus(ctx, workspaceID, "grooming"); err != nil {
		return err
	}
	session, err := u.pty.Open(prompt)
	if err != nil {
		u.repo.UpdateStatus(ctx, workspaceID, "idle")
		return fmt.Errorf("start grooming PTY: %w", err)
	}
	alias := filepath.Base(projectPath)
	u.lgSess.RegisterSession(workspaceID, alias, ws.ProjectID, session)
	return nil
}

func (u *WorkspaceUsecase) GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	return u.repo.GetTasks(ctx, workspaceID)
}

func (u *WorkspaceUsecase) ApproveCheckpoint(ctx context.Context, workspaceID string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if err := u.repo.ApproveTasks(ctx, workspaceID); err != nil {
		return err
	}
	return u.lgSess.RespondToCheckpoint(true, "")
}

func (u *WorkspaceUsecase) RejectCheckpoint(ctx context.Context, workspaceID, feedback string) error {
	if u.lgSess == nil {
		return fmt.Errorf("no active session")
	}
	if err := u.repo.DeletePendingTasks(ctx, workspaceID); err != nil {
		return err
	}
	return u.lgSess.RespondToCheckpoint(false, feedback)
}

func buildGroomingPrompt(ws entity.Workspace, projectPath string) string {
	const tmpl = `You are the Grooming model inside Lemongrass.

Your job is to understand the requirements below, reason about the codebase using the semantic map, and produce a task list that a separate model will later use to write the actual code. You do not write code yourself.

Requirements:
%s

--- Codebase navigation ---

Start with #lg.recon.tree to see the package map and annotation coverage.
Use #lg.recon.search to find relevant symbols by keyword -- this returns explored nodes with descriptions, which is faster than reading raw code.
When you find a gap (unexplored node you need to understand), use #lg.recon.read to get the raw code, then immediately fire #lg!.annotate so future sessions benefit.

--- Task list ---

When you have enough understanding, write a task list and call #lg.tasks.checkpoint.
Each task needs a title and a list of impl entries. An impl entry names a symbol, its file, and what needs to change -- rough and directional, not a code patch.

Example impl entry:
  "getByJob at modules/user/repo.go -- add tenant_id filter to WHERE clause"

The user will review the task list. On rejection you receive feedback -- revise and resubmit. When approved, call #lg!.handover to end this session.

--- Rules ---

Do not use shell commands (cat, ls, find, grep, git).
Do not write code. Your output is a task list.
Always annotate nodes you read -- the semantic map is shared across all sessions.
Call #lg!.handover only after #lg.tasks.checkpoint returns approved.`

	requirement := ws.RequirementText
	if ws.RequirementType == "pdf" || ws.RequirementType == "image" {
		alias := filepath.Base(projectPath)
		_ = alias
		runnerPath := "/home/lg/.lemongrass/workspaces/" + ws.ID + "/" + ws.RequirementFile
		if ws.RequirementType == "pdf" {
			requirement = "Your requirements are in the file at: " + runnerPath
		} else {
			requirement = "Your requirements are in the image file at: " + runnerPath
		}
	}
	return fmt.Sprintf(strings.TrimSpace(tmpl), requirement)
}

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
}

type draftStore interface {
	SaveDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error
	GetDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error)
	ClearDraft(ctx context.Context, workspaceID string) error
}

type lgSession interface {
	RegisterSession(workspaceID, projectAlias string, projectID int64, session *ptyclient.Session)
	RespondToCheckpoint(rejections map[string]string) error
}

type WorkspaceUsecase struct {
	repo   repo
	pty    *ptyclient.PtyClient
	lgSess lgSession
	draft  draftStore
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

func (u *WorkspaceUsecase) SetDraftStore(d draftStore) {
	u.draft = d
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
	systemPrompt := buildGroomingPrompt(ws, projectPath)
	if err := u.repo.UpdateStatus(ctx, workspaceID, "grooming"); err != nil {
		return err
	}
	session, err := u.pty.Open(systemPrompt)
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
	return u.lgSess.RespondToCheckpoint(nil)
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
	return u.lgSess.RespondToCheckpoint(rejections)
}

func buildGroomingPrompt(ws entity.Workspace, projectPath string) string {
	const tmpl = `Grooming model inside Lemongrass. Understand requirements, reason about codebase using semantic map, produce task list for execution model. No code generation.

Requirements:
%s

--- Navigation ---

#lg.recon.tree -- package map with annotation coverage. Start here.
#lg.recon.search <query> -- keyword search across explored nodes; faster than raw code.
#lg.recon.read <file:symbol:start-end> -- raw code for unexplored or stale nodes.
#lg.recon.related <symbol> -- callees and callers for explored symbols.
After #lg.recon.read, immediately call #lg!.annotate <file:symbol:start-end>:"desc":return_type (# is hook trigger, not a comment; ! = non-blocking fire-and-forget).

--- Tasks ---

After enough understanding, call #lg.tasks.checkpoint with:
{"tasks":[{"title":"...","impl":["symbol at file -- directive",...]},...]}

impl entry: symbol, file, what changes -- directional, not a patch.
Example: "getByJob at modules/user/repo.go -- add tenant_id filter to WHERE clause"

On rejection, receive per-task list:
  rejected:
  2: "Add TenantID migration" -- include index on tenant_id
Revise only rejected tasks, carry approved unchanged, resubmit full list.

--- Rules ---

Shell commands unavailable -- use lg protocol only.
Annotate every node you read -- semantic map shared across all sessions.
#lg.echo <message> as Bash tool call to surface blockers to user (# is hook trigger, not a comment).
#lg!.handover only after #lg.tasks.checkpoint returns approved.`

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

package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type amendmentSession interface {
	RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session)
}

type AmendmentUsecase struct {
	ws     workspaceStore
	req    requirementStore
	tasks  taskStore
	pty    ptyProvider
	lgSess amendmentSession
}

func NewAmendment(ws workspaceStore, req requirementStore, tasks taskStore, pty ptyProvider) *AmendmentUsecase {
	return &AmendmentUsecase{ws: ws, req: req, tasks: tasks, pty: pty}
}

func (u *AmendmentUsecase) SetLgSession(s amendmentSession) {
	u.lgSess = s
}

func (u *AmendmentUsecase) StartAmendmentSession(ctx context.Context, workspaceID string) error {
	if u.pty == nil || u.lgSess == nil {
		return fmt.Errorf("amendment not configured")
	}
	ws, err := u.ws.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if ws.Status != "grooming" && ws.Status != "amending" {
		return fmt.Errorf("workspace is %s, must be grooming or amending to start amendment", ws.Status)
	}
	allTasks, err := u.tasks.GetTasks(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}
	var approvedTasks, rejectedTasks []entity.Task
	for _, t := range allTasks {
		switch t.Status {
		case "approved":
			approvedTasks = append(approvedTasks, t)
		case "rejected":
			rejectedTasks = append(rejectedTasks, t)
		}
	}
	if len(rejectedTasks) == 0 {
		return fmt.Errorf("no rejected tasks to amend")
	}
	requirements, err := u.req.ListRequirements(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load requirements: %w", err)
	}
	requirements = convertRequirements(ctx, requirements)
	projectPath, err := u.ws.GetProjectPath(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	prompt := buildAmendmentPrompt(requirements, approvedTasks, rejectedTasks, projectPath)
	if err := u.ws.UpdateStatus(ctx, workspaceID, "amending"); err != nil {
		return err
	}
	session, err := u.pty.Open(prompt, workspaceID, "grooming")
	if err != nil {
		u.ws.UpdateStatus(ctx, workspaceID, "grooming")
		return fmt.Errorf("start amendment PTY: %w", err)
	}
	alias := filepath.Base(projectPath)
	u.lgSess.RegisterSession(workspaceID, alias, "grooming", ws.ProjectID, session)
	session.Write([]byte("Begin amendment.\r"))
	return nil
}

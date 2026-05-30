package usecase

import (
	"context"
	"fmt"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

func (u *LgUsecase) RegisterSession(workspaceID, projectAlias string, projectID int64, session ptyclient.Session) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.sessions[workspaceID] = &activeSession{
		workspaceID:  workspaceID,
		projectID:    projectID,
		projectAlias: projectAlias,
		ptySession:   session,
		checkpointCh: make(chan checkpointResult, 1),
	}
}

func (u *LgUsecase) RespondToCheckpoint(workspaceID string, rejections map[string]string) error {
	u.mu.Lock()
	s := u.sessions[workspaceID]
	u.mu.Unlock()
	if s == nil {
		return fmt.Errorf("no active session for workspace %s", workspaceID)
	}
	select {
	case s.checkpointCh <- checkpointResult{rejections: rejections}:
		return nil
	default:
		return fmt.Errorf("no pending checkpoint")
	}
}

func (u *LgUsecase) UnregisterSession(workspaceID string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.sessions, workspaceID)
}

func (u *LgUsecase) handleHandover(s *activeSession) {
	if u.tasks != nil {
		u.tasks.UpdateStatus(context.Background(), s.workspaceID, "awaiting_execution")
	}
	if s.ptySession != nil {
		s.ptySession.Close()
	}
	u.UnregisterSession(s.workspaceID)
}

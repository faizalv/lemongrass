package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

func (u *LgUsecase) HandleOrCreateSession(projectID int64, sessionID, cmd, args string, blocking bool) string {
	u.mu.Lock()
	if u.sessions[sessionID] == nil {
		u.sessions[sessionID] = &activeSession{
			key:            sessionID,
			projectID:      projectID,
			projectAlias:   fmt.Sprintf("project-%d", projectID),
			sessionType:    "headless",
			checkpointCh:   make(chan checkpointResult, 1),
			readNodes:      make(map[string]readEntry),
			commitments:    make(map[string]*commitment),
			taskStartTimes: make(map[string]time.Time),
		}
	}
	u.mu.Unlock()
	return u.Handle(sessionID, cmd, args, blocking)
}

func (u *LgUsecase) HandleByProject(projectID int64, cmd, args string, blocking bool) string {
	key := fmt.Sprintf("host:%d", projectID)
	u.mu.Lock()
	if u.sessions[key] == nil {
		u.sessions[key] = &activeSession{
			key:            key,
			projectID:      projectID,
			projectAlias:   fmt.Sprintf("project-%d", projectID),
			sessionType:    "host",
			checkpointCh:   make(chan checkpointResult, 1),
			readNodes:      make(map[string]readEntry),
			commitments:    make(map[string]*commitment),
			taskStartTimes: make(map[string]time.Time),
		}
	}
	u.mu.Unlock()
	return u.Handle(key, cmd, args, blocking)
}

func (u *LgUsecase) RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.sessions[workspaceID] = &activeSession{
		key:          workspaceID,
		workspaceID:  workspaceID,
		projectID:    projectID,
		projectAlias: projectAlias,
		sessionType:  sessionType,
		ptySession:   session,
		checkpointCh: make(chan checkpointResult, 1),
		readNodes:      make(map[string]readEntry),
		commitments:    make(map[string]*commitment),
		taskStartTimes: make(map[string]time.Time),
	}
	u.lastActivity[workspaceID] = time.Now()
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

func (u *LgUsecase) WriteToSession(workspaceID string, msg []byte) error {
	u.mu.Lock()
	s := u.sessions[workspaceID]
	u.mu.Unlock()
	if s == nil || s.ptySession == nil {
		return fmt.Errorf("no active session for workspace %s", workspaceID)
	}
	_, err := s.ptySession.Write(msg)
	return err
}

func (u *LgUsecase) UnregisterSession(workspaceID string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.sessions, workspaceID)
	delete(u.lastActivity, workspaceID)
}

func (u *LgUsecase) ResetSession(workspaceID string) {
	u.mu.Lock()
	s := u.sessions[workspaceID]
	u.mu.Unlock()
	if s != nil && s.ptySession != nil {
		s.ptySession.Close()
	}
	u.UnregisterSession(workspaceID)
}

func (u *LgUsecase) dropInterim(s *activeSession) {
	if u.interim != nil && s.key != "" {
		u.interim.DropInterim(context.Background(), s.key)
	}
}

func (u *LgUsecase) handleHandover(s *activeSession, args string) {
	if u.tasks != nil && u.recon != nil {
		ctx := context.Background()
		args = strings.TrimSpace(args)
		if args != "" {
			var lines []string
			for _, key := range strings.Split(args, ",") {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				content, err := u.recon.ReadKnowledge(ctx, s.projectID, key)
				if err == nil {
					lines = append(lines, key+": "+content)
				}
			}
			if len(lines) > 0 {
				u.tasks.SaveHandoverContext(ctx, s.workspaceID, strings.Join(lines, "\n"))
			}
		}
		u.tasks.UpdateStatus(ctx, s.workspaceID, "awaiting_execution")
	}
	u.dropInterim(s)
	if s.ptySession != nil {
		s.ptySession.Close()
	}
	u.UnregisterSession(s.workspaceID)
	go u.refreshUsageCache()
}

func (u *LgUsecase) handleDone(s *activeSession) {
	if u.tasks != nil {
		u.tasks.UpdateStatus(context.Background(), s.workspaceID, "done")
	}
	u.computeExecDiff(s.workspaceID)
	u.dropInterim(s)
	if s.ptySession != nil {
		s.ptySession.Close()
	}
	u.UnregisterSession(s.workspaceID)
	go u.refreshUsageCache()
}

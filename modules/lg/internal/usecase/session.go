package usecase

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

func (u *LgUsecase) HandleOrCreateSession(projectID int64, sessionID, cmd, args string, blocking bool) string {
	branch := u.resolveProjectBranch(projectID)
	u.mu.Lock()
	created := false
	if u.sessions[sessionID] == nil {
		u.sessions[sessionID] = &activeSession{
			key:            sessionID,
			projectID:      projectID,
			projectAlias:   fmt.Sprintf("project-%d", projectID),
			sessionType:    "headless",
			currentBranch:  branch,
			checkpointCh:   make(chan checkpointResult, 1),
			readNodes:      make(map[string]readEntry),
			writtenFiles:   make(map[string]bool),
			commitments:    make(map[string]*commitment),
			taskStartTimes: make(map[string]time.Time),
			warnedAt:       make(map[string]time.Time),
			obligation:     make(map[string]time.Time),
		}
		created = true
	}
	s := u.sessions[sessionID]
	u.mu.Unlock()
	if created {
		go u.persistHeadlessSession(s)
	}
	return u.Handle(sessionID, cmd, args, blocking)
}

func (u *LgUsecase) HandleByProject(projectID int64, cmd, args string, blocking bool) string {
	key := fmt.Sprintf("host:%d", projectID)
	branch := u.resolveProjectBranch(projectID)
	u.mu.Lock()
	if u.sessions[key] == nil {
		u.sessions[key] = &activeSession{
			key:            key,
			projectID:      projectID,
			projectAlias:   fmt.Sprintf("project-%d", projectID),
			sessionType:    "host",
			currentBranch:  branch,
			checkpointCh:   make(chan checkpointResult, 1),
			readNodes:      make(map[string]readEntry),
			writtenFiles:   make(map[string]bool),
			commitments:    make(map[string]*commitment),
			taskStartTimes: make(map[string]time.Time),
			warnedAt:       make(map[string]time.Time),
			obligation:     make(map[string]time.Time),
		}
	}
	u.mu.Unlock()
	return u.Handle(key, cmd, args, blocking)
}

func (u *LgUsecase) RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session) {
	branch := u.resolveProjectBranch(projectID)
	u.mu.Lock()
	defer u.mu.Unlock()
	u.sessions[workspaceID] = &activeSession{
		key:            workspaceID,
		workspaceID:    workspaceID,
		projectID:      projectID,
		projectAlias:   projectAlias,
		sessionType:    sessionType,
		currentBranch:  branch,
		ptySession:     session,
		checkpointCh:   make(chan checkpointResult, 1),
		readNodes:      make(map[string]readEntry),
		writtenFiles:   make(map[string]bool),
		commitments:    make(map[string]*commitment),
		taskStartTimes: make(map[string]time.Time),
		warnedAt:       make(map[string]time.Time),
		obligation:     make(map[string]time.Time),
	}
	u.lastActivity[workspaceID] = time.Now()
}

func (u *LgUsecase) resolveProjectBranch(projectID int64) string {
	if u.recon == nil {
		return "init"
	}
	rawPath, err := u.recon.ProjectDir(context.Background(), projectID)
	if err != nil {
		return "init"
	}
	dir := "/projects/" + filepath.Base(rawPath)
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "init"
	}
	b := strings.TrimSpace(string(out))
	if b == "" {
		return "init"
	}
	return b
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
	if s := u.sessions[workspaceID]; s != nil {
		releaseSessionLocks(s)
	}
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

type sessionRecord struct {
	Key        string    `json:"key"`
	ProjectID  int64     `json:"project_id"`
	LastActive time.Time `json:"last_active"`
}

func (u *LgUsecase) persistHeadlessSession(s *activeSession) {
	if u.recon == nil {
		return
	}
	rawPath, err := u.recon.ProjectDir(context.Background(), s.projectID)
	if err != nil || rawPath == "" {
		return
	}
	lgDir := filepath.Join("/projects", filepath.Base(rawPath), ".lemongrass")
	if err := os.MkdirAll(lgDir, 0o755); err != nil {
		return
	}
	sessionsPath := filepath.Join(lgDir, "sessions.jsonl")

	records := map[string]sessionRecord{}
	if f, err := os.Open(sessionsPath); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var rec sessionRecord
			if json.Unmarshal(sc.Bytes(), &rec) == nil {
				records[rec.Key] = rec
			}
		}
		f.Close()
	}

	now := time.Now()
	records[s.key] = sessionRecord{Key: s.key, ProjectID: s.projectID, LastActive: now}

	cutoff := now.Add(-7 * 24 * time.Hour)
	for key, rec := range records {
		if rec.LastActive.Before(cutoff) {
			delete(records, key)
		}
	}

	tmp, err := os.CreateTemp(lgDir, "sessions-*.jsonl")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	for _, rec := range records {
		enc.Encode(rec)
	}
	tmp.Close()
	os.Rename(tmpName, sessionsPath)

	u.mu.Lock()
	s.lastPersistedAt = now
	u.mu.Unlock()
}

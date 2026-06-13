package usecase

import (
	"context"
	"os"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/modules/lg/entity"
)

func (u *LgUsecase) LogWrite(sessionID, filePath string, byteCount int) {
	u.mu.Lock()
	u.writeTrail = append(u.writeTrail, entity.WriteTrailEntry{
		SessionID: sessionID,
		FilePath:  filePath,
		ByteCount: byteCount,
		Timestamp: time.Now(),
	})
	if len(u.writeTrail) > 200 {
		u.writeTrail = u.writeTrail[len(u.writeTrail)-200:]
	}
	s := u.sessions[sessionID]
	isExec := s != nil && s.sessionType == "execution"
	_, alreadySnapped := u.beforeSnapshots[sessionID][filePath]
	u.mu.Unlock()

	if isExec && !alreadySnapped {
		content, _ := os.ReadFile(filePath)
		u.mu.Lock()
		if u.beforeSnapshots[sessionID] == nil {
			u.beforeSnapshots[sessionID] = make(map[string]string)
		}
		if _, seen := u.beforeSnapshots[sessionID][filePath]; !seen {
			u.beforeSnapshots[sessionID][filePath] = string(content)
		}
		u.mu.Unlock()
	}

	if s != nil && u.recon != nil {
		u.recon.SyncGitProject(s.projectID)
	}
	if s != nil {
		bus.Default.Emit(bus.EventFileChanged, s.projectID)
	}
}

func (u *LgUsecase) LogRead(sessionID, filePath string) {
	if u.recon == nil {
		return
	}
	u.mu.Lock()
	s := u.sessions[sessionID]
	u.mu.Unlock()
	if s == nil {
		return
	}
	nodes, err := u.recon.ListFileNodes(context.Background(), s.projectID, filePath)
	if err != nil || len(nodes) == 0 {
		return
	}
	u.mu.Lock()
	for _, node := range nodes {
		key := filePath + ":" + node.Symbol + ":" + node.Kind
		if _, exists := s.readNodes[key]; !exists {
			s.readNodes[key] = readEntry{kind: node.Kind, signature: node.Signature}
		}
	}
	u.mu.Unlock()
}

func (u *LgUsecase) GetWriteTrail(sessionID string) []entity.WriteTrailEntry {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []entity.WriteTrailEntry
	for _, e := range u.writeTrail {
		if e.SessionID == sessionID {
			out = append(out, e)
		}
	}
	return out
}

func (u *LgUsecase) ListCalls() []entity.Call {
	u.mu.Lock()
	defer u.mu.Unlock()
	result := make([]entity.Call, len(u.calls))
	copy(result, u.calls)
	return result
}

func (u *LgUsecase) ListCallsByWorkspace(workspaceID string) []entity.Call {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []entity.Call
	for _, c := range u.calls {
		if c.SessionID == workspaceID {
			out = append(out, c)
		}
	}
	return out
}

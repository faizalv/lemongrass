package client

import (
	"time"

	"github.com/faizalv/lemongrass/modules/lg/entity"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
)

type SessionManager struct {
	uc *usecase.LgUsecase
}

func New(uc *usecase.LgUsecase) *SessionManager {
	return &SessionManager{uc: uc}
}

func (s *SessionManager) RegisterSession(workspaceID, projectAlias string, projectID int64, session ptyclient.Session) {
	s.uc.RegisterSession(workspaceID, projectAlias, projectID, session)
}

func (s *SessionManager) RespondToCheckpoint(workspaceID string, rejections map[string]string) error {
	return s.uc.RespondToCheckpoint(workspaceID, rejections)
}

func (s *SessionManager) UnregisterSession(workspaceID string) {
	s.uc.UnregisterSession(workspaceID)
}

func (s *SessionManager) GetSessionActivity(workspaceID string) (time.Time, int, []entity.EchoMessage, bool) {
	return s.uc.GetSessionActivity(workspaceID)
}

func (s *SessionManager) ResetSession(workspaceID string) {
	s.uc.ResetSession(workspaceID)
}

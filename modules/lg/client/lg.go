package client

import (
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
)

type SessionManager struct {
	uc *usecase.LgUsecase
}

func New(uc *usecase.LgUsecase) *SessionManager {
	return &SessionManager{uc: uc}
}

func (s *SessionManager) RegisterSession(workspaceID, projectAlias string, projectID int64, session *ptyclient.Session) {
	s.uc.RegisterSession(workspaceID, projectAlias, projectID, session)
}

func (s *SessionManager) RespondToCheckpoint(rejections map[string]string) error {
	return s.uc.RespondToCheckpoint(rejections)
}

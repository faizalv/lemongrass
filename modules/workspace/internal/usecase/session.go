package usecase

import (
	"context"
	"time"

	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
)

type sessionLgSession interface {
	GetSessionActivity(workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool)
	ResetSession(workspaceID string)
}

type SessionUsecase struct {
	ws     workspaceStore
	lgSess sessionLgSession
}

func NewSession(ws workspaceStore) *SessionUsecase {
	return &SessionUsecase{ws: ws}
}

func (u *SessionUsecase) SetLgSession(s sessionLgSession) {
	u.lgSess = s
}

func (u *SessionUsecase) GetSessionActivity(_ context.Context, workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool) {
	if u.lgSess == nil {
		return time.Time{}, -1, nil, false
	}
	return u.lgSess.GetSessionActivity(workspaceID)
}

func (u *SessionUsecase) ResetSession(ctx context.Context, workspaceID string) error {
	if u.lgSess != nil {
		u.lgSess.ResetSession(workspaceID)
	}
	return u.ws.UpdateStatus(ctx, workspaceID, "idle")
}

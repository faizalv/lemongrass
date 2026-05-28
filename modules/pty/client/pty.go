package client

import (
	"github.com/faizalv/lemongrass/modules/pty/internal/usecase"
)

type Session interface {
	Write(b []byte) (int, error)
	Close()
}

type PtyClient struct {
	uc *usecase.PtyUsecase
}

func New(uc *usecase.PtyUsecase) *PtyClient {
	return &PtyClient{uc: uc}
}

type session struct {
	inner *usecase.Session
}

func (s *session) Write(b []byte) (int, error) {
	return s.inner.Write(b)
}

func (s *session) Close() {
	s.inner.Close()
}

func (c *PtyClient) Open(prompt, sessionID, sessionType string) (Session, error) {
	s, err := c.uc.Open(prompt, sessionID, sessionType)
	if err != nil {
		return nil, err
	}
	return &session{inner: s}, nil
}

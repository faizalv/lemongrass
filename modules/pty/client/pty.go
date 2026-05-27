package client

import (
	"time"

	"github.com/faizalv/lemongrass/modules/pty/internal/usecase"
)

type PtyClient struct {
	uc *usecase.PtyUsecase
}

func New(uc *usecase.PtyUsecase) *PtyClient {
	return &PtyClient{uc: uc}
}

type Session struct {
	inner *usecase.Session
}

func (s *Session) Write(b []byte) (int, error) {
	return s.inner.Write(b)
}

func (s *Session) WaitIdle(quiesce, max time.Duration) {
	s.inner.WaitIdle(quiesce, max)
}

func (s *Session) Output() string {
	return s.inner.Output()
}

func (s *Session) Close() {
	s.inner.Close()
}

func (c *PtyClient) Open(prompt, sessionID, sessionType string) (*Session, error) {
	s, err := c.uc.Open(prompt, sessionID, sessionType)
	if err != nil {
		return nil, err
	}
	return &Session{inner: s}, nil
}

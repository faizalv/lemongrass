package client

import "github.com/faizalv/lemongrass/modules/pty/internal/usecase"

type PtyClient struct {
	uc *usecase.PtyUsecase
}

func New(uc *usecase.PtyUsecase) *PtyClient {
	return &PtyClient{uc: uc}
}

func (c *PtyClient) Run(prompt string) error {
	_, err := c.uc.Run(prompt)
	return err
}

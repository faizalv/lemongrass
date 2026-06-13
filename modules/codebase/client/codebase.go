package client

import (
	"context"

	"github.com/faizalv/lemongrass/modules/codebase/internal/usecase"
)

type Client struct {
	uc *usecase.CodebaseUsecase
}

func New(uc *usecase.CodebaseUsecase) *Client {
	return &Client{uc: uc}
}

func (c *Client) Ls(ctx context.Context, projectID int64, projectDir, args string) string {
	return c.uc.Ls(ctx, projectID, projectDir, args)
}

func (c *Client) Files(ctx context.Context, projectID int64, projectDir, args string) string {
	return c.uc.Files(ctx, projectID, projectDir, args)
}

func (c *Client) Search(ctx context.Context, projectID int64, projectDir string, filePaths []string, args string) string {
	return c.uc.Search(ctx, projectID, projectDir, filePaths, args)
}

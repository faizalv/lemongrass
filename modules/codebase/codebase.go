package codebase

import (
	"github.com/faizalv/lemongrass/config"
	cbclient "github.com/faizalv/lemongrass/modules/codebase/client"
	"github.com/faizalv/lemongrass/modules/codebase/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Codebase struct {
	uc *usecase.CodebaseUsecase
}

func (c *Codebase) LoadMe(_ config.Config, _ *sqlx.DB) {
	c.uc = usecase.New()
}

func (c *Codebase) StartHTTPRouter(_ *gin.RouterGroup) {}

func (c *Codebase) Client() *cbclient.Client {
	return cbclient.New(c.uc)
}

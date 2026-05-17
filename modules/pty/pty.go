package pty

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/faizalv/lemongrass/config"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	handler "github.com/faizalv/lemongrass/modules/pty/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/pty/internal/usecase"
)

type Pty struct {
	uc *usecase.PtyUsecase
	h  *handler.PtyHandler
}

func (p *Pty) LoadMe(cfg config.Config, db *sqlx.DB, rds *redis.Client) {
	p.uc = usecase.New("/var/log/lemongrass/runner.log")
	p.h = handler.New(p.uc)
}

func (p *Pty) Client() *ptyclient.PtyClient {
	return ptyclient.New(p.uc)
}

func (p *Pty) Close() {
	if p.uc != nil {
		p.uc.Close()
	}
}

func (p *Pty) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/pty")
	g.GET("/test", p.h.Test)
	g.POST("/send", p.h.Send)
}

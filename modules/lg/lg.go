package lg

import (
	"github.com/faizalv/lemongrass/config"
	handler "github.com/faizalv/lemongrass/modules/lg/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Lg struct {
	PtyClient *ptyclient.PtyClient
	h         *handler.LgHandler
}

func (l *Lg) LoadMe(cfg config.Config, db *sqlx.DB, rds *redis.Client) {
	uc := usecase.New(l.PtyClient)
	l.h = handler.New(uc)
}

func (l *Lg) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/lg")
	g.POST("", l.h.Receive)
	g.GET("/debug/calls", l.h.Calls)
	g.POST("/debug/send", l.h.Send)
}

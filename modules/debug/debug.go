package debug

import (
	"github.com/faizalv/lemongrass/config"
	handler "github.com/faizalv/lemongrass/modules/debug/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/debug/internal/usecase"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type ptyProvider interface {
	Open(prompt, sessionID, sessionType string) (ptyclient.Session, error)
	OpenNoop() ptyclient.Session
}

type sessionRegistrar interface {
	RegisterSession(workspaceID, projectAlias string, projectID int64, session ptyclient.Session)
	UnregisterSession(workspaceID string)
}

type Debug struct {
	PtyClient        ptyProvider
	SessionRegistrar sessionRegistrar
	h                *handler.DebugHandler
}

func (d *Debug) LoadMe(_ config.Config, _ *sqlx.DB) {
	d.h = handler.New(usecase.New(d.PtyClient), d.PtyClient, d.SessionRegistrar)
}

func (d *Debug) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/debug")
	g.POST("/exec", d.h.ExecHook)
	g.POST("/send", d.h.Send)
}

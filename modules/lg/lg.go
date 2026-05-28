package lg

import (
	"github.com/faizalv/lemongrass/config"
	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	handler "github.com/faizalv/lemongrass/modules/lg/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	reconclient "github.com/faizalv/lemongrass/modules/recon/client"
	wsclient "github.com/faizalv/lemongrass/modules/workspace/client"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Lg struct {
	PtyClient   *ptyclient.PtyClient
	ReconClient *reconclient.ReconClient
	uc          *usecase.LgUsecase
	h           *handler.LgHandler
}

func (l *Lg) LoadMe(_ config.Config, _ *sqlx.DB) {
	l.uc = usecase.New(l.PtyClient)
	if l.ReconClient != nil {
		l.uc.SetRecon(l.ReconClient)
	}
	l.h = handler.New(l.uc)
}

func (l *Lg) SetWorkspaceTaskClient(c *wsclient.WorkspaceTaskClient) {
	l.uc.SetTaskWriter(c)
}

func (l *Lg) SessionManager() *lgclient.SessionManager {
	return lgclient.New(l.uc)
}

func (l *Lg) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/lg")
	g.POST("", l.h.Receive)
	g.POST("/write-trail", l.h.WriteTrail)
	g.GET("/write-trail", l.h.GetWriteTrail)
	g.GET("/debug/calls", l.h.Calls)
	g.POST("/debug/send", l.h.Send)
}

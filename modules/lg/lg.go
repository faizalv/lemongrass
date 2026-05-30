package lg

import (
	"context"

	"github.com/faizalv/lemongrass/config"
	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	handler "github.com/faizalv/lemongrass/modules/lg/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type reconProvider interface {
	TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.DirectoryCoverage, error)
	ReadNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (reconentity.SemanticNode, string, error)
	Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) error
	Search(ctx context.Context, projectID int64, query string) ([]reconentity.SemanticNode, error)
	Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []reconentity.SemanticNode, err error)
	PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	SyncGitProject(projectID int64)
}

type taskProvider interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type Lg struct {
	ReconClient reconProvider
	uc          *usecase.LgUsecase
	h           *handler.LgHandler
}

func (l *Lg) LoadMe(_ config.Config, _ *sqlx.DB) {
	l.uc = usecase.New()
	if l.ReconClient != nil {
		l.uc.SetRecon(l.ReconClient)
	}
	l.h = handler.New(l.uc)
}

func (l *Lg) SetWorkspaceTaskClient(c taskProvider) {
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
	g.GET("/calls", l.h.Calls)
}

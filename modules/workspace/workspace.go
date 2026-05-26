package workspace

import (
	"github.com/faizalv/lemongrass/config"
	handler "github.com/faizalv/lemongrass/modules/workspace/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/workspace/internal/repository"
	"github.com/faizalv/lemongrass/modules/workspace/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Workspace struct {
	h *handler.WorkspaceHandler
}

func (w *Workspace) LoadMe(_ config.Config, db *sqlx.DB, _ *redis.Client) {
	repo := repository.New(db)
	uc := usecase.New(repo)
	w.h = handler.New(uc)
}

func (w *Workspace) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/workspaces")
	g.POST("", w.h.Create)
	g.GET("", w.h.ListByProject)
	g.GET("/:id", w.h.Get)
	g.POST("/:id/requirement", w.h.ReplaceRequirement)
}

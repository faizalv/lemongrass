package fs

import (
	"context"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/faizalv/lemongrass/config"
	handler "github.com/faizalv/lemongrass/modules/fs/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/fs/internal/repository"
	"github.com/faizalv/lemongrass/modules/fs/internal/usecase"
)

type Fs struct {
	h  *handler.FsHandler
	uc *usecase.FsUsecase
}

func (f *Fs) LoadMe(cfg config.Config, db *sqlx.DB, rds *redis.Client) {
	repo := repository.New(db)
	sockPath := filepath.Join(config.Dir(), "fs.sock")
	f.uc = usecase.New(repo, sockPath)
	f.h = handler.New(f.uc)
}

func (f *Fs) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/fs")
	g.GET("/browse", f.h.Browse)
	g.GET("/projects", f.h.ListProjects)
	g.POST("/projects", f.h.AddProject)
	g.DELETE("/projects/:id", f.h.DeleteProject)
}

func (f *Fs) Startup(ctx context.Context) {
	f.uc.RunSanityCheck(ctx)
}

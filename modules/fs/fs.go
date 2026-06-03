package fs

import (
	"context"
	"log"
	"path/filepath"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/modules/fs/entity"
	handler "github.com/faizalv/lemongrass/modules/fs/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/fs/internal/repository"
	"github.com/faizalv/lemongrass/modules/fs/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Fs struct {
	h  *handler.FsHandler
	uc *usecase.FsUsecase
}

func (f *Fs) LoadMe(cfg config.Config, db *sqlx.DB) {
	repo := repository.New(db)
	sockPath := filepath.Join(config.Dir(), "lemongrass.sock")
	f.uc = usecase.New(repo, sockPath)
	f.h = handler.New(f.uc)

	bus.Default.On(bus.EventProjectRemoved, func(payload any) {
		id, ok := payload.(int64)
		if !ok {
			return
		}
		if err := f.uc.DeleteArtifactsByProject(context.Background(), id); err != nil {
			log.Printf("fs: delete artifacts for project %d: %v", id, err)
		}
	})
}

func (f *Fs) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/fs")
	g.GET("/browse", f.h.Browse)
	g.GET("/projects", f.h.ListProjects)
	g.POST("/projects", f.h.AddProject)
	g.DELETE("/projects/:id", f.h.DeleteProject)
	g.GET("/projects/:id/artifacts", f.h.ListArtifacts)
	g.POST("/projects/:id/artifacts", f.h.CreateArtifact)
	g.DELETE("/projects/:id/artifacts/:artifact_id", f.h.DeleteArtifact)
}

func (f *Fs) Startup(ctx context.Context) {
	f.uc.RunSanityCheck(ctx)
}

func (f *Fs) ListProjects() ([]entity.Project, error) {
	return f.uc.ListProjects()
}

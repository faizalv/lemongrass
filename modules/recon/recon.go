package recon

import (
	"context"
	"log"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/config"
	handler "github.com/faizalv/lemongrass/modules/recon/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/recon/internal/repository"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/golang"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Recon struct {
	uc *usecase.ReconUsecase
	h  *handler.ReconHandler
}

func (r *Recon) LoadMe(_ config.Config, db *sqlx.DB, _ *redis.Client) {
	repo := repository.New(db)
	r.uc = usecase.New(repo, golang.New())
	r.h = handler.New(r.uc)

	bus.Default.On(bus.EventProjectRemoved, func(payload any) {
		id, ok := payload.(int64)
		if !ok {
			return
		}
		if err := repo.DeleteByProject(context.Background(), id); err != nil {
			log.Printf("recon: delete nodes for project %d: %v", id, err)
		}
	})
}

func (r *Recon) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/recon")
	g.GET("/projects/:id/nodes", r.h.ListNodes)
	g.GET("/projects/:id/coverage", r.h.GetCoverage)
	g.GET("/projects/:id/lgignore", r.h.GetLgIgnore)
}

func (r *Recon) MapIfNeeded(ctx context.Context, projectID int64, dir string) error {
	return r.uc.MapIfNeeded(ctx, projectID, dir)
}

func (r *Recon) Usecase() *usecase.ReconUsecase {
	return r.uc
}

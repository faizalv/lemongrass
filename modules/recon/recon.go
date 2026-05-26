package recon

import (
	"context"
	"log"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/config"
	reconclient "github.com/faizalv/lemongrass/modules/recon/client"
	handler "github.com/faizalv/lemongrass/modules/recon/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/recon/internal/repository"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	configparser "github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/config"
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
	r.uc = usecase.New(repo, golang.New(), configparser.New())
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
	g.POST("/projects/:id/activate", r.h.Activate)
	g.GET("/projects/:id/sync-status", r.h.SyncStatus)
	g.PATCH("/projects/:id/sync-interval", r.h.UpdateSyncInterval)
}

func (r *Recon) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.uc.TickScheduler(ctx)
			}
		}
	}()
}

func (r *Recon) MapIfNeeded(ctx context.Context, projectID int64, dir string) error {
	return r.uc.MapIfNeeded(ctx, projectID, dir)
}

func (r *Recon) Usecase() *usecase.ReconUsecase {
	return r.uc
}

func (r *Recon) Client() *reconclient.ReconClient {
	return reconclient.New(r.uc)
}

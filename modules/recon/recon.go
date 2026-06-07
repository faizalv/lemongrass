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
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
	configparser "github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/config"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/golang"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Recon struct {
	uc *usecase.ReconUsecase
	h  *handler.ReconHandler
}

func (r *Recon) LoadMe(cfg config.Config, db *sqlx.DB) {
	repo := repository.New(db)
	parsers := []lang.Parser{golang.New(), configparser.New(), lang.NewContainerParser(
		"http://lg-lang:3000", 80,
		func(dir string) bool { return true },
	)}
	r.uc = usecase.New(repo, "/var/log/lemongrass", parsers...)
	r.h = handler.New(r.uc)

	bus.Default.On(bus.EventProjectRemoved, func(payload any) {
		id, ok := payload.(int64)
		if !ok {
			return
		}
		if err := repo.DeleteByProject(context.Background(), id); err != nil {
			log.Printf("recon: delete nodes for project %d: %v", id, err)
		}
		if err := repo.DeleteKnowledgeByProject(context.Background(), id); err != nil {
			log.Printf("recon: delete knowledge for project %d: %v", id, err)
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
	g.GET("/projects/:id/git-status", r.h.GitStatus)
	g.POST("/projects/:id/git/init", r.h.GitInit)
	g.GET("/projects/:id/embed-status", r.h.EmbedStatus)
	g.GET("/projects/:id/knowledge", r.h.ListKnowledge)
}

func (r *Recon) StartScheduler(ctx context.Context) {
	r.uc.StartBackgroundEmbed(ctx)
	go func() {
		ticker60 := time.NewTicker(60 * time.Second)
		ticker5 := time.NewTicker(5 * time.Second)
		defer ticker60.Stop()
		defer ticker5.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker60.C:
				r.uc.TickScheduler(ctx)
			case <-ticker5.C:
				r.uc.TickGitPoller(ctx)
			}
		}
	}()
}

func (r *Recon) Close() {
	r.uc.Close()
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

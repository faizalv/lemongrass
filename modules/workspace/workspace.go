package workspace

import (
	"context"
	"log"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/config"
	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	handler "github.com/faizalv/lemongrass/modules/workspace/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/workspace/internal/repository"
	"github.com/faizalv/lemongrass/modules/workspace/internal/usecase"
	wsclient "github.com/faizalv/lemongrass/modules/workspace/client"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Workspace struct {
	PtyClient *ptyclient.PtyClient
	repo      *repository.WorkspaceRepository
	uc        *usecase.WorkspaceUsecase
	h         *handler.WorkspaceHandler
}

func (w *Workspace) LoadMe(_ config.Config, db *sqlx.DB) {
	w.repo = repository.New(db)
	w.uc = usecase.New(w.repo)
	if w.PtyClient != nil {
		w.uc.SetPty(w.PtyClient)
	}
	w.uc.SetDraftStore(repository.NewDraft())
	w.h = handler.New(w.uc)
	bus.Default.On(bus.EventProjectRemoved, func(payload any) {
		id, ok := payload.(int64)
		if !ok {
			return
		}
		if err := w.repo.DeleteByProject(context.Background(), id); err != nil {
			log.Printf("workspace: delete by project %d: %v", id, err)
		}
	})
}

func (w *Workspace) SetLgSession(s *lgclient.SessionManager) {
	w.uc.SetLgSession(s)
}

func (w *Workspace) TaskClient() *wsclient.WorkspaceTaskClient {
	return wsclient.New(w.repo)
}

func (w *Workspace) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/workspaces")
	g.POST("", w.h.Create)
	g.GET("", w.h.ListByProject)
	g.GET("/:id", w.h.Get)
	g.POST("/:id/groom", w.h.StartGrooming)
	g.GET("/:id/tasks", w.h.GetTasks)
	g.POST("/:id/tasks/approve", w.h.ApproveCheckpoint)
	g.GET("/:id/checkpoint/review/draft", w.h.GetCheckpointDraft)
	g.PUT("/:id/checkpoint/review/draft/:task_id", w.h.SaveTaskDecision)
	g.POST("/:id/checkpoint/review", w.h.SubmitCheckpointReviews)
	g.GET("/:id/requirements", w.h.ListRequirements)
	g.POST("/:id/requirements", w.h.AddRequirement)
	g.DELETE("/:id/requirements/:req_id", w.h.DeleteRequirement)
}

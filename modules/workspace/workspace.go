package workspace

import (
	"github.com/faizalv/lemongrass/config"
	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	handler "github.com/faizalv/lemongrass/modules/workspace/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/workspace/internal/repository"
	"github.com/faizalv/lemongrass/modules/workspace/internal/usecase"
	wsclient "github.com/faizalv/lemongrass/modules/workspace/client"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Workspace struct {
	PtyClient *ptyclient.PtyClient
	repo      *repository.WorkspaceRepository
	uc        *usecase.WorkspaceUsecase
	h         *handler.WorkspaceHandler
}

func (w *Workspace) LoadMe(_ config.Config, db *sqlx.DB, rds *redis.Client) {
	w.repo = repository.New(db)
	w.uc = usecase.New(w.repo)
	if w.PtyClient != nil {
		w.uc.SetPty(w.PtyClient)
	}
	w.uc.SetDraftStore(repository.NewDraft(rds))
	w.h = handler.New(w.uc)
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
	g.POST("/:id/requirement", w.h.ReplaceRequirement)
	g.POST("/:id/groom", w.h.StartGrooming)
	g.GET("/:id/tasks", w.h.GetTasks)
	g.POST("/:id/tasks/approve", w.h.ApproveCheckpoint)
	g.GET("/:id/checkpoint/review/draft", w.h.GetCheckpointDraft)
	g.PUT("/:id/checkpoint/review/draft/:task_id", w.h.SaveTaskDecision)
	g.POST("/:id/checkpoint/review", w.h.SubmitCheckpointReviews)
}

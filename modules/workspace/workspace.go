package workspace

import (
	"context"
	"log"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/config"
	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	wsclient "github.com/faizalv/lemongrass/modules/workspace/client"
	handler "github.com/faizalv/lemongrass/modules/workspace/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/workspace/internal/repository"
	"github.com/faizalv/lemongrass/modules/workspace/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type ptyProvider interface {
	Open(prompt, sessionID, sessionType string) (ptyclient.Session, error)
}

type lgSessionProvider interface {
	RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session)
	RespondToCheckpoint(workspaceID string, rejections map[string]string) error
	GetSessionActivity(workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool)
	ResetSession(workspaceID string)
}

type Workspace struct {
	PtyClient    ptyProvider
	repo         *repository.WorkspaceRepository
	groomUc      *usecase.GroomingUsecase
	checkpointUc *usecase.CheckpointUsecase
	sessUc       *usecase.SessionUsecase
	execUc       *usecase.ExecutionUsecase
	amendUc      *usecase.AmendmentUsecase
	h            *handler.WorkspaceHandler
}

func (w *Workspace) LoadMe(_ config.Config, db *sqlx.DB) {
	w.repo = repository.New(db)
	draft := repository.NewDraft()

	wsUc := usecase.NewWorkspace(w.repo)
	w.groomUc = usecase.NewGrooming(w.repo, w.repo, w.PtyClient)
	w.checkpointUc = usecase.NewCheckpoint(w.repo, w.repo, draft)
	reqUc := usecase.NewRequirement(w.repo, w.repo)
	w.sessUc = usecase.NewSession(w.repo)
	w.execUc = usecase.NewExecution(w.repo, w.PtyClient)
	w.amendUc = usecase.NewAmendment(w.repo, w.repo, w.repo, w.PtyClient)

	w.h = handler.New(wsUc, w.groomUc, w.checkpointUc, reqUc, w.sessUc, w.execUc, w.amendUc)

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

func (w *Workspace) SetLgSession(s lgSessionProvider) {
	w.groomUc.SetLgSession(s)
	w.checkpointUc.SetLgSession(s)
	w.sessUc.SetLgSession(s)
	w.execUc.SetLgSession(s)
	w.amendUc.SetLgSession(s)
}

func (w *Workspace) TaskClient() *wsclient.WorkspaceTaskClient {
	return wsclient.New(w.repo)
}

func (w *Workspace) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/workspaces")
	g.POST("", w.h.Create)
	g.GET("", w.h.ListByProject)
	g.GET("/:id", w.h.Get)
	g.DELETE("/:id", w.h.Delete)
	g.POST("/:id/groom", w.h.StartGrooming)
	g.GET("/:id/tasks", w.h.GetTasks)
	g.POST("/:id/tasks/approve", w.h.ApproveCheckpoint)
	g.GET("/:id/checkpoint/review/draft", w.h.GetCheckpointDraft)
	g.PUT("/:id/checkpoint/review/draft/:task_id", w.h.SaveTaskDecision)
	g.POST("/:id/checkpoint/review", w.h.SubmitCheckpointReviews)
	g.GET("/:id/requirements", w.h.ListRequirements)
	g.POST("/:id/requirements", w.h.AddRequirement)
	g.DELETE("/:id/requirements/:req_id", w.h.DeleteRequirement)
	g.GET("/:id/requirements/:req_id/file", w.h.ServeRequirementFile)
	g.GET("/:id/session/activity", w.h.SessionActivity)
	g.POST("/:id/session/reset", w.h.SessionReset)
	g.POST("/:id/execution/start", w.h.StartExecution)
	g.POST("/:id/execution/force-stop", w.h.ForceStopExecution)
	g.POST("/:id/amendment/start", w.h.StartAmendment)
}

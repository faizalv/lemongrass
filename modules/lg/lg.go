package lg

import (
	"context"

	"github.com/faizalv/lemongrass/config"
	lgclient "github.com/faizalv/lemongrass/modules/lg/client"
	handler "github.com/faizalv/lemongrass/modules/lg/internal/handler/http"
	"github.com/faizalv/lemongrass/modules/lg/internal/repository"
	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type reconProvider interface {
	TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.DirectoryCoverage, error)
	FindNodesBySymbol(ctx context.Context, projectID int64, filePath, symbol string) ([]reconentity.SemanticNode, error)
	ReadNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (reconentity.SemanticNode, string, error)
	Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) (int64, error)
	Search(ctx context.Context, projectID int64, query string) ([]reconentity.SemanticNode, error)
	Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []reconentity.SemanticNode, err error)
	PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, []reconentity.SubdirSummary, error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	ListAllNodesByPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, error)
	DropFile(ctx context.Context, projectID int64, path string)
	SyncGitProject(projectID int64)
	SaveKnowledge(ctx context.Context, projectID int64, key, content string, labels []string) (bool, error)
	ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error)
	SearchKnowledge(ctx context.Context, projectID int64, query, label string) ([]reconentity.KnowledgeEntry, bool, error)
	DeleteKnowledge(ctx context.Context, projectID int64, key string) (bool, error)
	FindSimilarKnowledge(ctx context.Context, projectID int64, content, excludeKey string) ([]string, error)
	UpsertLabel(ctx context.Context, projectID int64, label, content string) error
	FindSimilarLabels(ctx context.Context, projectID int64, label, content string) ([]string, error)
	ListAllLabels(ctx context.Context, projectID int64) ([]string, error)
	SearchLabels(ctx context.Context, projectID int64, query string) ([]string, error)
	SearchKnowledgeByLabel(ctx context.Context, projectID int64, label, query string) ([]reconentity.KnowledgeEntry, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	ProjectDir(ctx context.Context, projectID int64) (string, error)
	ListFileNodes(ctx context.Context, projectID int64, filePath string) ([]reconentity.SemanticNode, error)
	ListFilePaths(ctx context.Context, projectID int64) []string
	SyncStatus(projectID int64) (syncing bool, lastSyncedNano int64)
}

type taskProvider interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetTasks(ctx context.Context, workspaceID string) ([]wsentity.Task, error)
	SaveHandoverContext(ctx context.Context, workspaceID, context string) error
	CreateWorkspace(ctx context.Context, projectID int64, name string) (wsentity.Workspace, error)
	FindWorkspace(ctx context.Context, projectID int64, nameOrID string) (wsentity.Workspace, error)
}

type Lg struct {
	ReconClient reconProvider
	uc          *usecase.LgUsecase
	h           *handler.LgHandler
}

func (l *Lg) LoadMe(_ config.Config, db *sqlx.DB) {
	l.uc = usecase.New()
	if l.ReconClient != nil {
		l.uc.SetRecon(l.ReconClient)
	}
	if db != nil {
		l.uc.SetInterimRepo(repository.NewInterim(db))
	}
	l.h = handler.New(l.uc)
}

func (l *Lg) SetWorkspaceTaskClient(c taskProvider) {
	l.uc.SetTaskWriter(c)
}

type usageFetcher interface {
	FetchUsage() string
}

func (l *Lg) SetUsageProvider(p usageFetcher) {
	l.uc.SetUsageProvider(p)
}

func (l *Lg) StartUsageScheduler(ctx context.Context) {
	l.uc.StartUsageScheduler(ctx)
}

func (l *Lg) SessionManager() *lgclient.SessionManager {
	return lgclient.New(l.uc)
}

func (l *Lg) StartHTTPRouter(rg *gin.RouterGroup) {
	g := rg.Group("/lg")
	g.POST("", l.h.Receive)
	g.POST("/write-trail", l.h.WriteTrail)
	g.GET("/write-trail", l.h.GetWriteTrail)
	g.GET("/execution-diff", l.h.ExecutionDiff)
	g.GET("/usage", l.h.Usage)
	g.GET("/calls", l.h.Calls)
}

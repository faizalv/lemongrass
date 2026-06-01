package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/modules/lg/entity"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
)

type reconClient interface {
	TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.DirectoryCoverage, error)
	ReadNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (reconentity.SemanticNode, string, error)
	Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) error
	Search(ctx context.Context, projectID int64, query string) ([]reconentity.SemanticNode, error)
	Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []reconentity.SemanticNode, err error)
	PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	SyncGitProject(projectID int64)
}

type taskWriter interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type checkpointResult struct {
	rejections map[string]string
}

type activeSession struct {
	workspaceID  string
	projectID    int64
	projectAlias string
	ptySession   ptyclient.Session
	checkpointCh chan checkpointResult
}

type LgUsecase struct {
	recon        reconClient
	tasks        taskWriter
	mu           sync.Mutex
	calls        []entity.Call
	writeTrail   []entity.WriteTrailEntry
	sessions     map[string]*activeSession
	lastActivity map[string]time.Time
}

func New() *LgUsecase {
	uc := &LgUsecase{
		sessions:     make(map[string]*activeSession),
		lastActivity: make(map[string]time.Time),
	}
	bus.Default.On(bus.EventProjectRemoved, func(_ any) {
		uc.mu.Lock()
		uc.calls = nil
		uc.mu.Unlock()
	})
	return uc
}

func (u *LgUsecase) GetSessionActivity(workspaceID string) (lastAt time.Time, idleSec int, echoes []entity.EchoMessage, active bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	lastAt, active = u.lastActivity[workspaceID]
	if !active {
		idleSec = -1
		return
	}
	idleSec = int(time.Since(lastAt).Seconds())
	for _, c := range u.calls {
		if c.SessionID == workspaceID && c.Cmd == "echo" {
			echoes = append(echoes, entity.EchoMessage{Timestamp: c.Timestamp, Text: c.Args})
		}
	}
	if len(echoes) > 50 {
		echoes = echoes[len(echoes)-50:]
	}
	return
}

func (u *LgUsecase) SetRecon(r reconClient) {
	u.recon = r
}

func (u *LgUsecase) SetTaskWriter(tw taskWriter) {
	u.tasks = tw
}

func (u *LgUsecase) Handle(sessionID, cmd, args string, blocking bool) string {
	u.mu.Lock()
	u.calls = append(u.calls, entity.Call{Cmd: cmd, Args: args, SessionID: sessionID, Timestamp: time.Now()})
	if len(u.calls) > 200 {
		u.calls = u.calls[len(u.calls)-200:]
	}
	if sessionID != "" {
		u.lastActivity[sessionID] = time.Now()
	}
	s := u.sessions[sessionID]
	u.mu.Unlock()

	if cmd == "echo" {
		return args
	}

	if s == nil {
		return "error: no active session for this workspace"
	}
	if u.recon == nil {
		return "error: recon not available"
	}

	ctx := context.Background()
	switch cmd {
	case "recon.tree":
		return u.handleTree(ctx, s, args)
	case "recon.peek":
		return u.handlePeek(ctx, s, args)
	case "recon.search":
		return u.handleSearch(ctx, s, args)
	case "recon.read":
		return u.handleRead(ctx, s, args)
	case "recon.related":
		return u.handleRelated(ctx, s, args)
	case "annotate":
		go u.handleAnnotate(ctx, s, args)
		return ""
	case "tasks.checkpoint":
		return u.handleCheckpoint(ctx, s, args)
	case "handover":
		u.handleHandover(s)
		return ""
	}
	return ""
}

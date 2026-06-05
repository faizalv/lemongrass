package usecase

import (
	"context"
	"strings"
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
	DropFile(ctx context.Context, projectID int64, path string)
	SyncGitProject(projectID int64)
}

type taskWriter interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetTasks(ctx context.Context, workspaceID string) ([]wsentity.Task, error)
}

type checkpointResult struct {
	rejections map[string]string
}

type pendingNode struct {
	key      string // "filePath:symbol:kind"
	kind     string // "method" or "func"
	symbol   string
	filePath string
}

type domainObligation struct {
	pathPrefix      string
	nodes           []pendingNode
	annotatedKeys   map[string]bool
	methodsRequired int
	funcsRequired   int
	methodsMet      int
	funcsMet        int
}

type readEntry struct {
	kind      string
	signature string
	receiver  string
}

type activeSession struct {
	workspaceID  string
	projectID    int64
	projectAlias string
	sessionType  string
	ptySession   ptyclient.Session
	checkpointCh chan checkpointResult
	readNodes    map[string]readEntry            // "path:symbol:kind" -> entry
	peekDomains  map[string]*domainObligation    // path prefix -> obligation
}

type LgUsecase struct {
	recon           reconClient
	tasks           taskWriter
	usage           usageProvider
	mu              sync.Mutex
	calls           []entity.Call
	writeTrail      []entity.WriteTrailEntry
	sessions        map[string]*activeSession
	lastActivity    map[string]time.Time
	beforeSnapshots map[string]map[string]string
	execDiffs       map[string][]entity.FileDiff
}

func New() *LgUsecase {
	uc := &LgUsecase{
		sessions:        make(map[string]*activeSession),
		lastActivity:    make(map[string]time.Time),
		beforeSnapshots: make(map[string]map[string]string),
		execDiffs:       make(map[string][]entity.FileDiff),
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
		if c.SessionID != workspaceID {
			continue
		}
		if msg := activityMessage(c.Cmd, c.Args); msg != "" {
			echoes = append(echoes, entity.EchoMessage{Timestamp: c.Timestamp, Text: msg})
		}
	}
	if len(echoes) > 50 {
		echoes = echoes[len(echoes)-50:]
	}
	return
}

func activityMessage(cmd, args string) string {
	args = strings.TrimSpace(args)
	switch cmd {
	case "echo":
		return strings.Trim(args, `"'`)
	case "recon.tree":
		if args == "" {
			return "Checking project map"
		}
		return "Checking " + args
	case "recon.peek":
		return "Peeking at " + args
	case "recon.search":
		return "Searching for " + args
	case "recon.read":
		parts := strings.SplitN(args, ":", 3)
		if len(parts) >= 2 {
			return "Reading " + parts[1] + " in " + parts[0]
		}
		return "Reading " + args
	case "recon.related":
		parts := strings.SplitN(args, ":", 3)
		if len(parts) >= 2 {
			return "Checking relationships for " + parts[1]
		}
		return "Checking symbol relationships"
	case "annotate":
		parts := strings.SplitN(args, ":", 4)
		if len(parts) >= 2 {
			return "Annotating " + parts[1] + " in " + parts[0]
		}
		return "Annotating symbol"
	case "quota.status":
		return "Checking annotation quota"
	case "tasks.checkpoint":
		return "Sending task proposal"
	case "tasks.read":
		return "Reading task list"
	case "handover":
		return "Handing over to execution"
	case "done":
		return "Execution complete"
	}
	return ""
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
	case "recon.drop":
		go u.handleReconDrop(ctx, s, args)
		return ""
	case "annotate":
		go u.handleAnnotate(ctx, s, args)
		return ""
	case "quota.status":
		return u.handleQuotaStatus(s)
	case "tasks.checkpoint":
		return u.handleCheckpoint(ctx, s, args)
	case "tasks.read":
		return u.handleTasksRead(ctx, s)
	case "handover":
		go u.handleHandover(s)
		return ""
	case "done":
		go u.handleDone(s)
		return ""
	}
	return ""
}

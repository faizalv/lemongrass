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
	Annotate(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) (int64, error)
	Search(ctx context.Context, projectID int64, query string) ([]reconentity.SemanticNode, error)
	Related(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []reconentity.SemanticNode, err error)
	PeekDir(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, []reconentity.SubdirSummary, error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	ListAllNodesByPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.SemanticNode, error)
	DropFile(ctx context.Context, projectID int64, path string)
	SyncGitProject(projectID int64)
	SaveKnowledge(ctx context.Context, projectID int64, key, content string) error
	ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error)
	SearchKnowledge(ctx context.Context, projectID int64, query string) ([]reconentity.KnowledgeEntry, error)
}

type taskWriter interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
	GetTasks(ctx context.Context, workspaceID string) ([]wsentity.Task, error)
}

type checkpointResult struct {
	rejections map[string]string
}

type commitment struct {
	pathPrefix      string
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
	readNodes    map[string]readEntry        // "path:symbol:kind" -> entry
	commitments  map[string]*commitment      // path prefix -> commitment
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
	case "commitment":
		return "Declaring annotation commitment"
	case "commitment.status":
		return "Checking commitment status"
	case "tasks.checkpoint":
		return "Sending task proposal"
	case "tasks.read":
		return "Reading task list"
	case "handover":
		return "Handing over to execution"
	case "done":
		return "Execution complete"
	case "knowledge.save":
		if idx := strings.IndexByte(args, ':'); idx > 0 {
			return "Saving knowledge: " + strings.TrimSpace(args[:idx])
		}
		return "Saving knowledge entry"
	case "knowledge.search":
		return "Searching knowledge: " + args
	}
	return ""
}

func (u *LgUsecase) SetRecon(r reconClient) {
	u.recon = r
}

func (u *LgUsecase) SetTaskWriter(tw taskWriter) {
	u.tasks = tw
}

func (u *LgUsecase) logCall(sessionID, sessionType, cmd, args, response string, start time.Time) {
	c := entity.Call{
		Cmd:         cmd,
		Args:        args,
		Response:    response,
		SessionID:   sessionID,
		SessionType: sessionType,
		DurationMs:  time.Since(start).Milliseconds(),
		Timestamp:   start,
	}
	u.mu.Lock()
	u.calls = append(u.calls, c)
	if len(u.calls) > 200 {
		u.calls = u.calls[len(u.calls)-200:]
	}
	u.mu.Unlock()
}

func (u *LgUsecase) Handle(sessionID, cmd, args string, blocking bool) string {
	start := time.Now()
	u.mu.Lock()
	if sessionID != "" {
		u.lastActivity[sessionID] = start
	}
	s := u.sessions[sessionID]
	u.mu.Unlock()

	if cmd == "echo" {
		u.logCall(sessionID, "", cmd, args, args, start)
		return args
	}

	if s == nil {
		resp := "error: no active session for this workspace"
		u.logCall(sessionID, "", cmd, args, resp, start)
		return resp
	}
	if u.recon == nil {
		resp := "error: recon not available"
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	}

	ctx := context.Background()
	switch cmd {
	case "recon.tree":
		resp := u.handleTree(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "recon.peek":
		resp := u.handlePeek(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "recon.search":
		resp := u.handleSearch(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "recon.read":
		resp := u.handleRead(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "recon.related":
		resp := u.handleRelated(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "recon.drop":
		go func() {
			u.handleReconDrop(ctx, s, args)
			u.logCall(sessionID, s.sessionType, cmd, args, "ok", start)
		}()
		return ""
	case "annotate":
		go func() {
			resp := u.handleAnnotate(ctx, s, args)
			u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		}()
		return ""
	case "commitment":
		resp := u.handleCommitment(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "commitment.status":
		resp := u.handleCommitmentStatus(s)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "tasks.checkpoint":
		resp := u.handleCheckpoint(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "tasks.read":
		resp := u.handleTasksRead(ctx, s)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "handover":
		go func() {
			u.handleHandover(s)
			u.logCall(sessionID, s.sessionType, cmd, args, "ok", start)
		}()
		return ""
	case "done":
		go func() {
			u.handleDone(s)
			u.logCall(sessionID, s.sessionType, cmd, args, "ok", start)
		}()
		return ""
	case "knowledge.save":
		resp := u.handleKnowledgeSave(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "knowledge.read":
		resp := u.handleKnowledgeRead(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	case "knowledge.search":
		resp := u.handleKnowledgeSearch(ctx, s, args)
		u.logCall(sessionID, s.sessionType, cmd, args, resp, start)
		return resp
	}
	return ""
}

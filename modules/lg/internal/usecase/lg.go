package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/modules/lg/entity"
	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
)

type reconUsecase interface {
	TreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]reconentity.DirectoryCoverage, error)
	ReadNode(ctx context.Context, projectID int64, filePath, symbol string) (reconentity.SemanticNode, string, error)
	Annotate(ctx context.Context, projectID int64, filePath, symbol, description, returnType string, calls []string) error
	Search(ctx context.Context, projectID int64, query string) ([]reconentity.SemanticNode, error)
	Related(ctx context.Context, projectID int64, symbol string) (callees, callers []reconentity.SemanticNode, err error)
}

type taskWriter interface {
	CreateTasks(ctx context.Context, workspaceID string, tasks []wsentity.Task) ([]wsentity.Task, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type checkpointResult struct {
	rejections map[string]string // task UUID -> feedback; empty = all approved
}

type activeSession struct {
	workspaceID  string
	projectID    int64
	projectAlias string
	ptySession   *ptyclient.Session
	checkpointCh chan checkpointResult
}

type LgUsecase struct {
	recon      reconUsecase
	tasks      taskWriter
	mu         sync.Mutex
	calls      []entity.Call
	writeTrail []entity.WriteTrailEntry
	sessions   map[string]*activeSession
}

func New() *LgUsecase {
	uc := &LgUsecase{sessions: make(map[string]*activeSession)}
	bus.Default.On(bus.EventProjectRemoved, func(_ any) {
		uc.mu.Lock()
		uc.calls = nil
		uc.mu.Unlock()
	})
	return uc
}

func (u *LgUsecase) SetRecon(r reconUsecase) {
	u.recon = r
}

func (u *LgUsecase) SetTaskWriter(tw taskWriter) {
	u.tasks = tw
}

func (u *LgUsecase) RegisterSession(workspaceID, projectAlias string, projectID int64, session *ptyclient.Session) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.sessions[workspaceID] = &activeSession{
		workspaceID:  workspaceID,
		projectID:    projectID,
		projectAlias: projectAlias,
		ptySession:   session,
		checkpointCh: make(chan checkpointResult, 1),
	}
}

func (u *LgUsecase) RespondToCheckpoint(workspaceID string, rejections map[string]string) error {
	u.mu.Lock()
	s := u.sessions[workspaceID]
	u.mu.Unlock()
	if s == nil {
		return fmt.Errorf("no active session for workspace %s", workspaceID)
	}
	select {
	case s.checkpointCh <- checkpointResult{rejections: rejections}:
		return nil
	default:
		return fmt.Errorf("no pending checkpoint")
	}
}

func (u *LgUsecase) Handle(sessionID, cmd, args string, blocking bool) string {
	u.mu.Lock()
	u.calls = append(u.calls, entity.Call{Cmd: cmd, Args: args, Timestamp: time.Now()})
	if len(u.calls) > 200 {
		u.calls = u.calls[len(u.calls)-200:]
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

func (u *LgUsecase) handleTree(ctx context.Context, s *activeSession, args string) string {
	dirs, err := u.recon.TreeCoverage(ctx, s.projectID, strings.TrimSpace(args))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(dirs) == 0 {
		return "no nodes found"
	}
	var sb strings.Builder
	for _, d := range dirs {
		sb.WriteString(fmt.Sprintf("%-50s %3d nodes  %3d explored  %3d stale  %3d unexplored\n",
			d.Dir, d.Total, d.Explored, d.Stale, d.Unexplored))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleSearch(ctx context.Context, s *activeSession, query string) string {
	nodes, err := u.recon.Search(ctx, s.projectID, strings.TrimSpace(query))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(nodes) == 0 {
		return "no results"
	}
	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(formatAnnotate(n))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleRead(ctx context.Context, s *activeSession, args string) string {
	filePath, symbol, lineStart, lineEnd, err := parseRef(strings.TrimSpace(args))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	node, code, err := u.recon.ReadNode(ctx, s.projectID, filePath, symbol)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	hint := ""
	if node.Status == "stale" && node.Description != "" {
		hint = "[STALE] " + node.Description + "\n\n"
	}
	return fmt.Sprintf("%s:%s:%d-%d:\n%s%s",
		filePath, symbol, lineStart, lineEnd, hint, code)
}

func (u *LgUsecase) handleRelated(ctx context.Context, s *activeSession, symbol string) string {
	callees, callers, err := u.recon.Related(ctx, s.projectID, strings.TrimSpace(symbol))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- calls (%s calls these):\n", symbol))
	if len(callees) == 0 {
		sb.WriteString("(none found)\n")
	} else {
		for _, n := range callees {
			sb.WriteString(formatAnnotate(n))
			sb.WriteByte('\n')
		}
	}
	sb.WriteString(fmt.Sprintf("\n-- called by (these call %s):\n", symbol))
	if len(callers) == 0 {
		sb.WriteString("(none found)\n")
	} else {
		for _, n := range callers {
			sb.WriteString(formatAnnotate(n))
			sb.WriteByte('\n')
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleAnnotate(ctx context.Context, s *activeSession, args string) {
	filePath, symbol, _, _, description, returnType, calls, err := parseAnnotateFormat(args)
	if err != nil {
		return
	}
	u.recon.Annotate(ctx, s.projectID, filePath, symbol, description, returnType, calls)
}

func (u *LgUsecase) handleCheckpoint(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	var payload struct {
		Tasks []struct {
			Title string   `json:"title"`
			Impl  []string `json:"impl"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(args)), &payload); err != nil {
		return fmt.Sprintf("error: invalid tasks JSON: %v", err)
	}

	tasks := make([]wsentity.Task, len(payload.Tasks))
	for i, t := range payload.Tasks {
		implJSON, _ := json.Marshal(t.Impl)
		tasks[i] = wsentity.Task{
			WorkspaceID: s.workspaceID,
			Title:       t.Title,
			Impl:        implJSON,
		}
	}
	created, err := u.tasks.CreateTasks(ctx, s.workspaceID, tasks)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	result := <-s.checkpointCh
	if len(result.rejections) == 0 {
		return "approved"
	}
	var sb strings.Builder
	sb.WriteString("rejected:\n")
	for i, t := range created {
		if feedback, ok := result.rejections[t.ID]; ok {
			sb.WriteString(fmt.Sprintf("%d: %q -- %s\n", i+1, t.Title, feedback))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleHandover(s *activeSession) {
	if u.tasks != nil {
		u.tasks.UpdateStatus(context.Background(), s.workspaceID, "awaiting_execution")
	}
	if s.ptySession != nil {
		s.ptySession.Close()
	}
	u.mu.Lock()
	delete(u.sessions, s.workspaceID)
	u.mu.Unlock()
}

func (u *LgUsecase) LogWrite(sessionID, filePath string, byteCount int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.writeTrail = append(u.writeTrail, entity.WriteTrailEntry{
		SessionID: sessionID,
		FilePath:  filePath,
		ByteCount: byteCount,
		Timestamp: time.Now(),
	})
	if len(u.writeTrail) > 200 {
		u.writeTrail = u.writeTrail[len(u.writeTrail)-200:]
	}
}

func (u *LgUsecase) GetWriteTrail(sessionID string) []entity.WriteTrailEntry {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []entity.WriteTrailEntry
	for _, e := range u.writeTrail {
		if e.SessionID == sessionID {
			out = append(out, e)
		}
	}
	return out
}

func (u *LgUsecase) ListCalls() []entity.Call {
	u.mu.Lock()
	defer u.mu.Unlock()
	result := make([]entity.Call, len(u.calls))
	copy(result, u.calls)
	return result
}

func formatAnnotate(n reconentity.SemanticNode) string {
	desc := n.Description
	if n.Status == "stale" {
		desc = "[STALE] " + desc
	}
	calls := ""
	if len(n.Calls) > 0 {
		calls = ":[" + strings.Join(n.Calls, ",") + "]"
	}
	return fmt.Sprintf("%s:%s:%d-%d:\"%s\":%s%s",
		n.FilePath, n.Symbol, n.LineStart, n.LineEnd, desc, n.ReturnType, calls)
}

func parseRef(s string) (filePath, symbol string, lineStart, lineEnd int, err error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 3 {
		err = fmt.Errorf("expected file:symbol:start-end, got %q", s)
		return
	}
	filePath = parts[0]
	symbol = parts[1]
	lineStart, lineEnd, err = parseLineRange(parts[2])
	return
}

func parseLineRange(s string) (start, end int, err error) {
	idx := strings.Index(s, "-")
	if idx < 0 {
		err = fmt.Errorf("invalid line range %q", s)
		return
	}
	start, err = strconv.Atoi(s[:idx])
	if err != nil {
		return
	}
	end, err = strconv.Atoi(s[idx+1:])
	return
}

func parseAnnotateFormat(s string) (filePath, symbol string, lineStart, lineEnd int, description, returnType string, calls []string, err error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 4 {
		err = fmt.Errorf("invalid annotate format")
		return
	}
	filePath = parts[0]
	symbol = parts[1]
	lineStart, lineEnd, err = parseLineRange(parts[2])
	if err != nil {
		return
	}

	rest := parts[3]
	if !strings.HasPrefix(rest, `"`) {
		err = fmt.Errorf("expected quoted description")
		return
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, `"`)
	if closeIdx < 0 {
		err = fmt.Errorf("unclosed description quote")
		return
	}
	description = rest[:closeIdx]
	rest = rest[closeIdx+1:]

	if !strings.HasPrefix(rest, ":") {
		return
	}
	rest = rest[1:]

	if bracketIdx := strings.LastIndex(rest, ":["); bracketIdx >= 0 {
		returnType = rest[:bracketIdx]
		callStr := strings.TrimSuffix(rest[bracketIdx+2:], "]")
		for _, c := range strings.Split(callStr, ",") {
			if t := strings.TrimSpace(c); t != "" {
				calls = append(calls, t)
			}
		}
	} else if strings.HasPrefix(rest, "[") {
		callStr := strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]")
		for _, c := range strings.Split(callStr, ",") {
			if t := strings.TrimSpace(c); t != "" {
				calls = append(calls, t)
			}
		}
	} else {
		returnType = rest
	}
	return
}

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
)

func kindWeight(kind string) int {
	switch kind {
	case "method", "func":
		return 3
	case "struct", "type", "interface", "dockerfile", "makefile", "ci-github", "ci-gitlab", "compose", "config-yaml":
		return 2
	case "imports":
		return 0
	default:
		return 1
	}
}

func quotaRequired(weightedUnexplored int) int {
	const pct     = 0.10
	const minFloor = 5
	const maxCap   = 30
	req := int(math.Ceil(float64(weightedUnexplored) * pct))
	if req < minFloor {
		return minFloor
	}
	if req > maxCap {
		return maxCap
	}
	return req
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

func stripProjectPrefix(projectAlias, path string) string {
	prefix := "/projects/" + projectAlias + "/"
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

func (u *LgUsecase) handlePeek(ctx context.Context, s *activeSession, args string) string {
	pathPrefix := stripProjectPrefix(s.projectAlias, strings.TrimSpace(args))
	if pathPrefix == "" {
		return "error: recon.peek requires a directory path"
	}
	nodes, err := u.recon.PeekDir(ctx, s.projectID, pathPrefix)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(nodes) == 0 {
		return "no symbols found under " + pathPrefix
	}

	type fileGroup struct {
		path    string
		regular []reconentity.SemanticNode
		imports []reconentity.SemanticNode
	}
	var files []fileGroup
	fileIdx := map[string]int{}
	for _, n := range nodes {
		idx, ok := fileIdx[n.FilePath]
		if !ok {
			idx = len(files)
			files = append(files, fileGroup{path: n.FilePath})
			fileIdx[n.FilePath] = idx
		}
		if n.Kind == "imports" {
			files[idx].imports = append(files[idx].imports, n)
		} else {
			files[idx].regular = append(files[idx].regular, n)
		}
	}

	var sb strings.Builder
	for i, f := range files {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(f.path + "\n")
		all := append(f.regular, f.imports...)
		for _, n := range all {
			sym := n.Symbol
			if n.Kind == "method" && n.Receiver != "" {
				sym = n.Receiver + "." + n.Symbol
			}
			marker := ""
			switch n.Status {
			case "stale":
				marker = "   *stale"
			case "unexplored":
				marker = "   ?unexplored"
			}
			sb.WriteString(fmt.Sprintf("  %-8s %-44s %d-%d%s\n",
				n.Kind, sym, n.LineStart, n.LineEnd, marker))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleSearch(ctx context.Context, s *activeSession, query string) string {
	total, explored, err := u.recon.GetProjectCoverage(ctx, s.projectID)
	if err == nil && total > 0 {
		pct := explored * 100 / total
		if pct < 80 {
			return fmt.Sprintf("error: coverage too low to search (%d%% explored) -- use recon.peek + recon.read to build the map first", pct)
		}
	}
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
	filePath, symbol, kind, err := parseRef(strings.TrimSpace(args))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	filePath = stripProjectPrefix(s.projectAlias, filePath)
	node, code, err := u.recon.ReadNode(ctx, s.projectID, filePath, symbol, kind)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	s.readNodes[filePath+":"+symbol+":"+node.Kind] = node.Kind
	u.mu.Unlock()
	hint := ""
	if node.Status == "stale" && node.Description != "" {
		hint = "[STALE] " + node.Description + "\nLast annotated before code change. Re-read and re-annotate.\n---\n"
	}
	return fmt.Sprintf("%s:%s:%s:\n%s%s", filePath, symbol, kind, hint, code)
}

func (u *LgUsecase) handleRelated(ctx context.Context, s *activeSession, args string) string {
	filePath, symbol, kind, err := parseRef(strings.TrimSpace(args))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	filePath = stripProjectPrefix(s.projectAlias, filePath)
	callees, callers, err := u.recon.Related(ctx, s.projectID, filePath, symbol, kind)
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

func (u *LgUsecase) handleTasksRead(ctx context.Context, s *activeSession) string {
	if u.tasks == nil {
		return "error: task store not available"
	}
	tasks, err := u.tasks.GetTasks(ctx, s.workspaceID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	approved := make([]wsentity.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Status == "approved" {
			approved = append(approved, t)
		}
	}
	if len(approved) == 0 {
		return "no approved tasks found"
	}
	type taskOut struct {
		Title  string          `json:"title"`
		Reason string          `json:"reason"`
		Impl   json.RawMessage `json:"impl"`
	}
	out := make([]taskOut, len(approved))
	for i, t := range approved {
		out[i] = taskOut{Title: t.Title, Reason: t.Reason, Impl: t.Impl}
	}
	b, err := json.MarshalIndent(map[string]any{"tasks": out}, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

func (u *LgUsecase) handleAnnotate(ctx context.Context, s *activeSession, args string) {
	filePath, symbol, kind, description, returnType, calls, err := parseAnnotateFormat(args)
	if err != nil {
		return
	}
	filePath = stripProjectPrefix(s.projectAlias, filePath)
	key := filePath + ":" + symbol + ":" + kind
	u.mu.Lock()
	if readKind, ok := s.readNodes[key]; ok {
		s.annotationScore += kindWeight(readKind)
	}
	u.mu.Unlock()
	u.recon.Annotate(ctx, s.projectID, filePath, symbol, kind, description, returnType, calls)
}

func (u *LgUsecase) handleCheckpoint(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	var payload struct {
		Tasks []struct {
			Title  string   `json:"title"`
			Reason string   `json:"reason"`
			Impl   []string `json:"impl"`
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
			Reason:      t.Reason,
			Impl:        implJSON,
		}
	}
	if weighted, err := u.recon.GetWeightedUnexplored(ctx, s.projectID); err == nil {
		required := quotaRequired(weighted)
		u.mu.Lock()
		score := s.annotationScore
		u.mu.Unlock()
		if score < required {
			return fmt.Sprintf(
				"annotation quota not met: %d/%d pts -- earn %d more.\n"+
					"Priority: method/func (3pts) > struct/type/interface/config (2pts) > const/var (1pt).\n"+
					"Read a node via #lg.recon.read first, then annotate. Unread annotations score 0.",
				score, required, required-score,
			)
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

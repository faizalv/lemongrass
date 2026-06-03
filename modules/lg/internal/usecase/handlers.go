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

const coverageRate = 0.30

func domainThreshold(n int) int {
	return int(math.Ceil(float64(n) * coverageRate))
}

func bestMatchDomain(peekDomains map[string]*domainObligation, filePath string) *domainObligation {
	var best *domainObligation
	bestLen := 0
	for prefix, ob := range peekDomains {
		norm := strings.TrimSuffix(prefix, "/")
		if (strings.HasPrefix(filePath, norm+"/") || filePath == norm) && len(norm) > bestLen {
			best = ob
			bestLen = len(norm)
		}
	}
	return best
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

func stripReceiver(symbol string) string {
	if i := strings.LastIndex(symbol, "."); i >= 0 {
		return symbol[i+1:]
	}
	return symbol
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

	u.mu.Lock()
	if _, exists := s.peekDomains[pathPrefix]; !exists {
		ob := &domainObligation{
			pathPrefix:    pathPrefix,
			annotatedKeys: make(map[string]bool),
		}
		for _, n := range nodes {
			if n.Status != "unexplored" {
				continue
			}
			switch n.Kind {
			case "method", "func":
				key := n.FilePath + ":" + n.Symbol + ":" + n.Kind
				ob.nodes = append(ob.nodes, pendingNode{
					key:      key,
					kind:     n.Kind,
					symbol:   n.Symbol,
					filePath: n.FilePath,
				})
				if n.Kind == "method" {
					ob.methodsRequired++
				} else {
					ob.funcsRequired++
				}
			}
		}
		s.peekDomains[pathPrefix] = ob
	}
	u.mu.Unlock()

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
	nodes, err := u.recon.Search(ctx, s.projectID, strings.TrimSpace(query))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	coverageNote := ""
	if total, explored, err := u.recon.GetProjectCoverage(ctx, s.projectID); err == nil && total > 0 {
		pct := explored * 100 / total
		coverageNote = fmt.Sprintf(" (%d%% of %d nodes annotated)", pct, total)
	}

	if len(nodes) == 0 {
		return "no results" + coverageNote
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- %d result(s)%s\n", len(nodes), coverageNote))
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
	symbol = stripReceiver(symbol)
	node, code, err := u.recon.ReadNode(ctx, s.projectID, filePath, symbol, kind)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	s.readNodes[filePath+":"+symbol+":"+node.Kind] = readEntry{
		kind:      node.Kind,
		signature: node.Signature,
		receiver:  node.Receiver,
	}
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
	symbol = stripReceiver(symbol)
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

func (u *LgUsecase) handleQuotaStatus(s *activeSession) string {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(s.peekDomains) == 0 {
		return "no obligations yet -- peek a directory first"
	}

	var sb strings.Builder
	for _, ob := range s.peekDomains {
		mThresh := domainThreshold(ob.methodsRequired)
		fThresh := domainThreshold(ob.funcsRequired)
		met := ob.methodsMet >= mThresh && (fThresh == 0 || ob.funcsMet >= fThresh)
		status := "PENDING"
		if met {
			status = "OK"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s  methods %d/%d  funcs %d/%d\n",
			status, ob.pathPrefix, ob.methodsMet, mThresh, ob.funcsMet, fThresh))
		for _, n := range ob.nodes {
			if ob.annotatedKeys[n.key] {
				continue
			}
			if entry, read := s.readNodes[n.key]; read {
				sig := entry.signature
				if entry.receiver != "" && sig != "" {
					sig = "(" + entry.receiver + ") " + sig
				} else if entry.receiver != "" {
					sig = "(" + entry.receiver + ")"
				}
				sb.WriteString(fmt.Sprintf("  %-8s  %-36s  %s  [annotate from memory]\n", n.kind, n.symbol, sig))
			} else {
				sb.WriteString(fmt.Sprintf("  %-8s  %-36s  [not read]\n", n.kind, n.symbol))
			}
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
	symbol = stripReceiver(symbol)
	key := filePath + ":" + symbol + ":" + kind
	u.mu.Lock()
	if entry, ok := s.readNodes[key]; ok {
		if ob := bestMatchDomain(s.peekDomains, filePath); ob != nil {
			switch entry.kind {
			case "method":
				ob.methodsMet++
				ob.annotatedKeys[key] = true
			case "func":
				ob.funcsMet++
				ob.annotatedKeys[key] = true
			}
		}
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
	u.mu.Lock()
	obligations := make([]*domainObligation, 0, len(s.peekDomains))
	for _, ob := range s.peekDomains {
		obligations = append(obligations, ob)
	}
	u.mu.Unlock()
	for _, ob := range obligations {
		mThresh := domainThreshold(ob.methodsRequired)
		fThresh := domainThreshold(ob.funcsRequired)
		if mThresh > 0 && ob.methodsMet < mThresh {
			return fmt.Sprintf(
				"annotation quota not met for %s:\n  methods: %d/%d annotated (need %d more)\nRead method bodies -- they reveal how this domain is written.",
				ob.pathPrefix, ob.methodsMet, mThresh, mThresh-ob.methodsMet,
			)
		}
		if fThresh > 0 && ob.funcsMet < fThresh {
			return fmt.Sprintf(
				"annotation quota not met for %s:\n  funcs: %d/%d annotated (need %d more)",
				ob.pathPrefix, ob.funcsMet, fThresh, fThresh-ob.funcsMet,
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

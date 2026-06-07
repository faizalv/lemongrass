package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
	wsentity "github.com/faizalv/lemongrass/modules/workspace/entity"
)

const coverageRate          = 0.30
const commitmentMethodCap   = 15
const commitmentFuncCap     = 8
const commitmentRootThreshold = 0.70

func commitmentThreshold(n, cap int) int {
	t := int(math.Ceil(float64(n) * coverageRate))
	if t > cap {
		return cap
	}
	return t
}

func bestMatchCommitment(commitments map[string]*commitment, filePath string) *commitment {
	var best *commitment
	bestLen := 0
	for prefix, c := range commitments {
		if prefix == "." {
			if best == nil {
				best = c
			}
			continue
		}
		norm := strings.TrimSuffix(prefix, "/")
		if (strings.HasPrefix(filePath, norm+"/") || filePath == norm) && len(norm) > bestLen {
			best = c
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
		if d.Stale > 0 {
			sb.WriteString(fmt.Sprintf("%-50s %d/%d explored; %d stale\n", d.Dir, d.Explored, d.Total, d.Stale))
		} else {
			sb.WriteString(fmt.Sprintf("%-50s %d/%d explored\n", d.Dir, d.Explored, d.Total))
		}
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
	nodes, subdirs, err := u.recon.PeekDir(ctx, s.projectID, pathPrefix)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(nodes) == 0 && len(subdirs) == 0 {
		return "no symbols found under " + pathPrefix
	}


	var sb strings.Builder
	for _, sd := range subdirs {
		sb.WriteString(fmt.Sprintf("%-50s %d symbols\n", sd.Path, sd.Count))
	}
	if len(subdirs) > 0 && len(nodes) > 0 {
		sb.WriteByte('\n')
	}
	if len(nodes) == 0 {
		return strings.TrimRight(sb.String(), "\n")
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
	if len(nodes) == 0 {
		return "no results"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- %d result(s)\n", len(nodes)))
	for _, n := range nodes {
		sb.WriteString(formatAnnotate(n))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleRead(ctx context.Context, s *activeSession, args string) string {
	refs := strings.Split(args, "|")
	if len(refs) == 1 {
		return u.readOne(ctx, s, strings.TrimSpace(refs[0]))
	}
	var parts []string
	for i, ref := range refs {
		result := u.readOne(ctx, s, strings.TrimSpace(ref))
		parts = append(parts, fmt.Sprintf("==== [%d] ====\n%s", i+1, result))
	}
	return strings.Join(parts, "\n")
}

func (u *LgUsecase) readOne(ctx context.Context, s *activeSession, ref string) string {
	filePath, symbol, kind, err := parseRef(ref)
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

func (u *LgUsecase) handleCommitment(ctx context.Context, s *activeSession, args string) string {
	path := strings.TrimPrefix(strings.TrimSpace(args), "./")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "."
	}

	if path == "." {
		total, explored, err := u.recon.GetProjectCoverage(ctx, s.projectID)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if total > 0 && float64(explored)/float64(total) < commitmentRootThreshold {
			pct := explored * 100 / total
			return fmt.Sprintf("project is %d%% explored; reach 70%% before committing to root", pct)
		}
	}

	u.mu.Lock()
	_, exists := s.commitments[path]
	u.mu.Unlock()
	if exists {
		return "already committed to " + path
	}

	nodes, err := u.recon.ListAllNodesByPrefix(ctx, s.projectID, path)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	var methodCount, funcCount int
	for _, n := range nodes {
		if n.Status != "unexplored" {
			continue
		}
		switch reconentity.KindRole(n.Kind) {
		case "method":
			methodCount++
		case "func":
			funcCount++
		}
	}

	mThresh := commitmentThreshold(methodCount, commitmentMethodCap)
	fThresh := commitmentThreshold(funcCount, commitmentFuncCap)

	c := &commitment{
		pathPrefix:      path,
		annotatedKeys:   make(map[string]bool),
		methodsRequired: methodCount,
		funcsRequired:   funcCount,
	}
	u.mu.Lock()
	s.commitments[path] = c
	u.mu.Unlock()

	if methodCount == 0 && funcCount == 0 {
		return "committed to " + path + ": no unexplored methods or funcs found; checkpoint unlocked for this path"
	}
	return fmt.Sprintf("committed to %s: %d unexplored methods (need %d), %d unexplored funcs (need %d)",
		path, methodCount, mThresh, funcCount, fThresh)
}

func (u *LgUsecase) handleCommitmentStatus(s *activeSession) string {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(s.commitments) == 0 {
		return "no commitments yet -- use #lg.commitment <path> to declare what you will annotate"
	}

	var sb strings.Builder
	for _, c := range s.commitments {
		mThresh := commitmentThreshold(c.methodsRequired, commitmentMethodCap)
		fThresh := commitmentThreshold(c.funcsRequired, commitmentFuncCap)
		met := c.methodsMet >= mThresh && (fThresh == 0 || c.funcsMet >= fThresh)
		status := "PENDING"
		if met {
			status = "OK"
		}
		sb.WriteString(fmt.Sprintf("[%s] %-40s  methods %d/%d  funcs %d/%d\n",
			status, c.pathPrefix, c.methodsMet, mThresh, c.funcsMet, fThresh))
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

func (u *LgUsecase) handleReconDrop(ctx context.Context, s *activeSession, args string) {
	rel := stripProjectPrefix(s.projectAlias, strings.TrimSpace(args))
	if rel == "" {
		return
	}
	abs := "/projects/" + s.projectAlias + "/" + rel
	if !strings.HasPrefix(abs, "/projects/"+s.projectAlias+"/") {
		return
	}
	os.Remove(abs)
	u.recon.DropFile(ctx, s.projectID, rel)
}

func (u *LgUsecase) handleKnowledgeSave(ctx context.Context, s *activeSession, args string) string {
	idx := strings.IndexByte(args, ':')
	if idx <= 0 {
		return "error: format is knowledge.save <key>:<content>"
	}
	key := strings.TrimSpace(args[:idx])
	content := strings.TrimSpace(args[idx+1:])
	if content == "" {
		return "error: content is empty"
	}
	if err := u.recon.SaveKnowledge(ctx, s.projectID, key, content); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	similar, _ := u.recon.FindSimilarKnowledge(ctx, s.projectID, content, key)
	if len(similar) > 0 {
		return "saved: " + key + " [similar: " + strings.Join(similar, ", ") + "]"
	}
	return "saved: " + key
}

func (u *LgUsecase) handleKnowledgeRead(ctx context.Context, s *activeSession, args string) string {
	key := strings.TrimSpace(args)
	content, err := u.recon.ReadKnowledge(ctx, s.projectID, key)
	if err != nil {
		return "not found: " + key
	}
	return content
}

func (u *LgUsecase) handleKnowledgeSearch(ctx context.Context, s *activeSession, args string) string {
	entries, err := u.recon.SearchKnowledge(ctx, s.projectID, strings.TrimSpace(args))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(entries) == 0 {
		return "no knowledge entries found"
	}
	var sb strings.Builder
	for _, e := range entries {
		snippet := e.Content
		if len(snippet) > 120 {
			snippet = snippet[:120] + "…"
		}
		sb.WriteString(e.Key + ": " + snippet + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleKnowledgeDelete(ctx context.Context, s *activeSession, args string) string {
	key := strings.TrimSpace(args)
	if key == "" {
		return "error: key required"
	}
	deleted, err := u.recon.DeleteKnowledge(ctx, s.projectID, key)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if !deleted {
		return "not found: " + key
	}
	return "deleted: " + key
}

func (u *LgUsecase) handleAnnotate(ctx context.Context, s *activeSession, args string) string {
	filePath, symbol, kind, description, returnType, calls, err := parseAnnotateFormat(args)
	if err != nil {
		return "error: " + err.Error()
	}
	filePath = stripProjectPrefix(s.projectAlias, filePath)
	symbol = stripReceiver(symbol)
	key := filePath + ":" + symbol + ":" + kind
	u.mu.Lock()
	if entry, ok := s.readNodes[key]; ok {
		if c := bestMatchCommitment(s.commitments, filePath); c != nil {
			switch reconentity.KindRole(entry.kind) {
			case "method":
				c.methodsMet++
				c.annotatedKeys[key] = true
			case "func":
				c.funcsMet++
				c.annotatedKeys[key] = true
			}
		}
	}
	u.mu.Unlock()
	n, err := u.recon.Annotate(ctx, s.projectID, filePath, symbol, kind, description, returnType, calls)
	if err != nil {
		return "error: " + err.Error()
	}
	if n == 0 {
		return "not found: " + symbol + " (" + kind + ")"
	}
	return "ok"
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
	commitments := make([]*commitment, 0, len(s.commitments))
	for _, c := range s.commitments {
		commitments = append(commitments, c)
	}
	u.mu.Unlock()
	if len(commitments) == 0 {
		total, explored, err := u.recon.GetProjectCoverage(ctx, s.projectID)
		if err != nil || explored < total {
			return "no commitment made -- use #lg.commitment <path> to declare what you will annotate"
		}
	}
	for _, c := range commitments {
		mThresh := commitmentThreshold(c.methodsRequired, commitmentMethodCap)
		fThresh := commitmentThreshold(c.funcsRequired, commitmentFuncCap)
		if mThresh > 0 && c.methodsMet < mThresh {
			return fmt.Sprintf(
				"commitment not met for %s:\n  methods: %d/%d annotated (need %d more)\nRead method bodies -- they reveal how this domain is written.",
				c.pathPrefix, c.methodsMet, mThresh, mThresh-c.methodsMet,
			)
		}
		if fThresh > 0 && c.funcsMet < fThresh {
			return fmt.Sprintf(
				"commitment not met for %s:\n  funcs: %d/%d annotated (need %d more)",
				c.pathPrefix, c.funcsMet, fThresh, fThresh-c.funcsMet,
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

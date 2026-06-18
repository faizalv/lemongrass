package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/modules/lg/entity"
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

func (u *LgUsecase) handleProjectStat(ctx context.Context, s *activeSession) string {
	total, explored, _ := u.recon.GetProjectCoverage(ctx, s.projectID)
	dir, _ := u.recon.ProjectDir(ctx, s.projectID)
	device := config.LoadDevice()

	var sb strings.Builder
	if dir != "" {
		sb.WriteString("project: " + filepath.Base(dir) + "\n")
	}

	pct := 0
	if total > 0 {
		pct = 100 * explored / total
		sb.WriteString(fmt.Sprintf("coverage: %d%% (%d/%d explored)\n", pct, explored, total))
	} else {
		sb.WriteString("coverage: 0% (no symbols mapped yet)\n")
	}

	if device.Tier != "unknown" {
		sb.WriteString(fmt.Sprintf("device: %s (%dMB RAM, %d cores)\n", device.Tier, device.MemoryMB, device.CPUCores))
	} else {
		sb.WriteString("device: unknown (run lemongrass up to detect)\n")
	}

	sb.WriteString("advice: ")
	switch {
	case pct >= 60:
		sb.WriteString("recon.search is primary; use codebase.interim for unexplored areas")
	case pct >= 20:
		sb.WriteString("partial coverage -- recon.search across all signatures; codebase.interim+query for deeper unexplored areas")
	default:
		sb.WriteString("annotation sparse -- recon.search works on signatures from day 0; codebase.interim+query for full file context")
	}

	return strings.TrimRight(sb.String(), "\n")
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
		return "error: recon.peek requires a directory or file path"
	}

	if filepath.Ext(pathPrefix) != "" {
		nodes, err := u.recon.ListFileNodes(ctx, s.projectID, pathPrefix)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if len(nodes) == 0 {
			return "no symbols found in " + pathPrefix
		}
		return formatPeekNodes(nodes)
	}

	nodes, subdirs, err := u.recon.PeekDir(ctx, s.projectID, pathPrefix)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(nodes) == 0 && len(subdirs) == 0 {
		return "no symbols found under " + pathPrefix
	}


	var sb strings.Builder
	sb.WriteString(strings.TrimSuffix(pathPrefix, "/") + "/\n")
	for _, sd := range subdirs {
		name := filepath.Base(sd.Path) + "/"
		sb.WriteString(fmt.Sprintf("  %-24s %d symbols\n", name, sd.Count))
	}
	if len(nodes) > 0 {
		sb.WriteString(formatPeekDir(pathPrefix, nodes))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatPeekDir(dirPrefix string, nodes []reconentity.SemanticNode) string {
	type fileGroup struct {
		name    string
		regular []reconentity.SemanticNode
		imports []reconentity.SemanticNode
	}
	var files []fileGroup
	fileIdx := map[string]int{}
	prefix := strings.TrimSuffix(dirPrefix, "/") + "/"
	for _, n := range nodes {
		name := strings.TrimPrefix(n.FilePath, prefix)
		idx, ok := fileIdx[name]
		if !ok {
			idx = len(files)
			files = append(files, fileGroup{name: name})
			fileIdx[name] = idx
		}
		if n.Kind == "imports" {
			files[idx].imports = append(files[idx].imports, n)
		} else {
			files[idx].regular = append(files[idx].regular, n)
		}
	}
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString("  " + f.name + "\n")
		for _, n := range append(f.regular, f.imports...) {
			sb.WriteString(formatSymbolLine("    ", n))
		}
	}
	return sb.String()
}

func formatPeekNodes(nodes []reconentity.SemanticNode) string {
	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(formatSymbolLine("  ", n))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatSymbolLine(indent string, n reconentity.SemanticNode) string {
	sym := n.Symbol
	if n.Kind == "method" && n.Receiver != "" {
		sym = n.Receiver + "." + n.Symbol
	}
	marker := ""
	switch n.Status {
	case "stale":
		marker = "  *"
	case "unexplored":
		marker = "  ?"
	}
	return fmt.Sprintf("%s%-9s %-36s %d-%d%s\n", indent, n.Kind, sym, n.LineStart, n.LineEnd, marker)
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
	var allRefs []string
	for _, group := range strings.Split(args, "||") {
		allRefs = append(allRefs, expandRefs(strings.TrimSpace(group))...)
	}
	if len(allRefs) == 1 {
		return u.readOne(ctx, s, allRefs[0])
	}
	var parts []string
	for i, ref := range allRefs {
		result := u.readOne(ctx, s, ref)
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
		if alts, altErr := u.recon.FindNodesBySymbol(ctx, s.projectID, filePath, symbol); altErr == nil && len(alts) > 0 {
			var sb strings.Builder
			fmt.Fprintf(&sb, "%s not found as %s -- did you mean:\n", symbol, kind)
			for _, a := range alts {
				sym := a.Symbol
				if a.Receiver != "" {
					sym = a.Receiver + "." + a.Symbol
				}
				marker := ""
				if a.Status == "stale" {
					marker = "   *stale"
				}
				fmt.Fprintf(&sb, "  %-8s %-44s %d-%d%s\n", a.Kind, sym, a.LineStart, a.LineEnd, marker)
			}
			return strings.TrimRight(sb.String(), "\n")
		}
		syncing, _ := u.recon.SyncStatus(s.projectID)
		if syncing {
			return "not in semantic map yet (recon is still scanning) -- use system.read or codebase.search in the meantime; check recon.tree for coverage once scanning finishes"
		}
		return "not in semantic map -- use system.read or codebase.search; check recon.tree for coverage"
	}
	u.mu.Lock()
	key := filePath + ":" + symbol + ":" + node.Kind
	s.readNodes[key] = readEntry{
		kind:      node.Kind,
		signature: node.Signature,
		receiver:  node.Receiver,
		readAt:    time.Now(),
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
		TaskNum int             `json:"task_id"`
		Title   string          `json:"title"`
		Reason  string          `json:"reason"`
		Impl    json.RawMessage `json:"impl"`
	}
	out := make([]taskOut, len(approved))
	for i, t := range approved {
		out[i] = taskOut{TaskNum: t.TaskNumber, Title: t.Title, Reason: t.Reason, Impl: t.Impl}
	}
	b, err := json.MarshalIndent(map[string]any{"tasks": out}, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

func rejectionNote(tasks []wsentity.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	var parts []string
	for _, t := range tasks {
		parts = append(parts, fmt.Sprintf("%q: %s", t.Title, t.RejectionReason))
	}
	return "rejection pending: " + strings.Join(parts, "; ")
}

func (u *LgUsecase) handleTasksStart(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task store not available"
	}
	if s.workspaceID == "" {
		return "error: no workspace active -- call #lg.workspace.use <name> first"
	}
	num, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil || num < 1 {
		return "error: task_id must be a positive integer"
	}
	task, err := u.tasks.GetTaskByNumber(ctx, s.workspaceID, num)
	if err != nil {
		return fmt.Sprintf("error: task %d not found", num)
	}
	now := time.Now()
	if err := u.tasks.StartTask(ctx, task.ID, now); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	s.taskStartTimes[task.ID] = now
	u.mu.Unlock()

	rejected, _ := u.tasks.GetRejectedTasks(ctx, s.workspaceID)
	resp := "ok: " + task.Title
	if note := rejectionNote(rejected); note != "" {
		resp += " -- " + note
	}
	return resp
}

func (u *LgUsecase) handleTasksFinish(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task store not available"
	}
	if s.workspaceID == "" {
		return "error: no workspace active -- call #lg.workspace.use <name> first"
	}
	sep := strings.IndexByte(args, ':')
	var numStr, notes string
	if sep < 0 {
		numStr = strings.TrimSpace(args)
	} else {
		numStr = strings.TrimSpace(args[:sep])
		notes = strings.TrimSpace(args[sep+1:])
	}
	num, err := strconv.Atoi(numStr)
	if err != nil || num < 1 {
		return "error: task_id must be a positive integer -- format: <task_id>:<notes>"
	}
	task, err := u.tasks.GetTaskByNumber(ctx, s.workspaceID, num)
	if err != nil {
		return fmt.Sprintf("error: task %d not found", num)
	}

	u.mu.Lock()
	startTime, hasStart := s.taskStartTimes[task.ID]
	snapshots := u.beforeSnapshots[s.workspaceID]
	var trailPaths []string
	seen := make(map[string]bool)
	for _, e := range u.writeTrail {
		if e.SessionID == s.workspaceID && !e.Timestamp.Before(startTime) && !seen[e.FilePath] {
			trailPaths = append(trailPaths, e.FilePath)
			seen[e.FilePath] = true
		}
	}
	u.mu.Unlock()

	_ = hasStart
	var diffs []entity.FileDiff
	for _, path := range trailPaths {
		before := snapshots[path]
		afterBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		diffs = append(diffs, buildDiff(before, string(afterBytes), path))
	}

	now := time.Now()
	if err := u.tasks.FinishTask(ctx, task.ID, notes, diffs, now); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	rejected, _ := u.tasks.GetRejectedTasks(ctx, s.workspaceID)
	resp := "done: " + task.Title
	if note := rejectionNote(rejected); note != "" {
		resp += " -- " + note + ". Finish all in-progress work, then address the rejection before calling #lg!.done."
	}
	return resp
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

func parseKnowledgeLabels(content string) (string, []string) {
	idx := strings.LastIndex(content, " ")
	if idx < 0 {
		return content, nil
	}
	last := content[idx+1:]
	if !strings.ContainsRune(last, ',') {
		return content, nil
	}
	var labels []string
	for _, l := range strings.Split(last, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			labels = append(labels, l)
		}
	}
	if len(labels) == 0 {
		return content, nil
	}
	return strings.TrimSpace(content[:idx]), labels
}

func parseKnowledgeSearchArgs(args string) (query, label string) {
	args = strings.TrimSpace(args)
	idx := strings.IndexByte(args, ':')
	if idx < 0 {
		return args, ""
	}
	return strings.TrimSpace(args[:idx]), strings.TrimSpace(args[idx+1:])
}

func (u *LgUsecase) handleKnowledgeSave(ctx context.Context, s *activeSession, args string) string {
	idx := strings.IndexByte(args, ':')
	if idx <= 0 {
		return "error: format is knowledge.save <key>:<content>"
	}
	key := strings.TrimSpace(args[:idx])
	raw := strings.TrimSpace(args[idx+1:])
	if raw == "" {
		return "error: content is empty"
	}
	content, labels := parseKnowledgeLabels(raw)
	embedded, err := u.recon.SaveKnowledge(ctx, s.projectID, key, content, labels)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	var signals []string
	if !embedded {
		signals = append(signals, "[warning: embedding unavailable -- search will use substring fallback until embed service recovers]")
	}
	similar, _ := u.recon.FindSimilarKnowledge(ctx, s.projectID, content, key)
	if len(similar) > 0 {
		signals = append(signals, "[similar: "+strings.Join(similar, ", ")+"]")
	}
	for _, label := range labels {
		_ = u.recon.UpsertLabel(ctx, s.projectID, label, content)
		similarLabels, _ := u.recon.FindSimilarLabels(ctx, s.projectID, label, content)
		if len(similarLabels) > 0 {
			signals = append(signals, "[similar labels: "+strings.Join(similarLabels, ", ")+"]")
		}
	}
	if len(signals) > 0 {
		return "saved: " + key + " " + strings.Join(signals, " ")
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
	query, label := parseKnowledgeSearchArgs(args)
	entries, fallback, err := u.recon.SearchKnowledge(ctx, s.projectID, query, label)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(entries) == 0 {
		return "no knowledge entries found"
	}
	var sb strings.Builder
	if fallback {
		sb.WriteString("[fallback: embed unavailable, results are substring matches not semantic]\n")
	}
	for _, e := range entries {
		snippet := e.Content
		if len(snippet) > 120 {
			snippet = snippet[:120] + "…"
		}
		sb.WriteString(e.Key + ": " + snippet + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleKnowledgeLabels(ctx context.Context, s *activeSession, args string) string {
	args = strings.TrimSpace(args)
	var labels []string
	var err error
	if args == "" {
		labels, err = u.recon.ListAllLabels(ctx, s.projectID)
	} else {
		labels, err = u.recon.SearchLabels(ctx, s.projectID, args)
	}
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(labels) == 0 {
		return "no labels yet"
	}
	return strings.Join(labels, "\n")
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

func (u *LgUsecase) handleObligation(s *activeSession) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(s.obligation) == 0 {
		return "no obligation"
	}
	elapsed := time.Since(s.obligationStart)
	remaining := 5*time.Minute - elapsed
	var sb strings.Builder
	if remaining > 0 {
		fmt.Fprintf(&sb, "obligation: %d symbol(s) -- annotate within %ds\n", len(s.obligation), int(remaining.Seconds()))
	} else {
		fmt.Fprintf(&sb, "obligation: %d symbol(s) -- OVERDUE by %ds\n", len(s.obligation), int(-remaining.Seconds()))
	}
	for key := range s.obligation {
		fmt.Fprintf(&sb, "  #lg.annotate %s:\"description\":nil:nil\n", key)
	}
	return strings.TrimRight(sb.String(), "\n")
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
	u.mu.Lock()
	delete(s.obligation, key)
	delete(s.readNodes, key)
	if len(s.obligation) == 0 {
		s.obligationStart = time.Time{}
	}
	u.mu.Unlock()
	return "ok"
}

func (u *LgUsecase) handleWorkspaceCreate(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	name := strings.TrimSpace(args)
	if name == "" {
		return "error: workspace name required"
	}
	ws, err := u.tasks.CreateWorkspace(ctx, s.projectID, name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	s.workspaceID = ws.ID
	u.mu.Unlock()
	return "workspace ready: " + ws.Name + " (" + ws.ID + ")"
}

func (u *LgUsecase) handleWorkspaceRequirementAdd(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	if s.workspaceID == "" {
		return "error: no workspace active -- call #lg.workspace.create <name> or #lg.workspace.use <name> first"
	}
	ws, err := u.tasks.GetWorkspace(ctx, s.workspaceID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if ws.Status != "idle" && ws.Status != "grooming" {
		return fmt.Sprintf("error: workspace is %s -- create a new workspace to amend", workspaceStatus(ws.Status))
	}
	text := strings.TrimSpace(args)
	if text == "" {
		return "error: requirement text required"
	}
	if err := u.tasks.AddTextRequirement(ctx, s.workspaceID, text); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "requirement added"
}

func workspaceStatus(raw string) string {
	switch raw {
	case "awaiting_execution":
		return "waiting execution"
	default:
		return raw
	}
}

func formatWorkspaces(workspaces []wsentity.Workspace) string {
	if len(workspaces) == 0 {
		return "no workspaces found"
	}
	var sb strings.Builder
	for _, ws := range workspaces {
		fmt.Fprintf(&sb, "%s  %s  %s\n", ws.Name, ws.CreatedAt.Format("2006-01-02"), workspaceStatus(ws.Status))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (u *LgUsecase) handleWorkspaceList(ctx context.Context, s *activeSession) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	workspaces, err := u.tasks.ListWorkspaces(ctx, s.projectID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return formatWorkspaces(workspaces)
}

func (u *LgUsecase) handleWorkspaceSearch(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	query := strings.ToLower(strings.TrimSpace(args))
	if query == "" {
		return "error: search query required"
	}
	workspaces, err := u.tasks.ListWorkspaces(ctx, s.projectID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	var matched []wsentity.Workspace
	for _, ws := range workspaces {
		if strings.Contains(strings.ToLower(ws.Name), query) {
			matched = append(matched, ws)
		}
	}
	return formatWorkspaces(matched)
}

func (u *LgUsecase) handleWorkspaceDelete(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	nameOrID := strings.TrimSpace(args)
	if nameOrID == "" {
		return "error: workspace name or ID required"
	}
	ws, err := u.tasks.FindWorkspace(ctx, s.projectID, nameOrID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if err := u.tasks.DeleteWorkspace(ctx, ws.ID); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	if s.workspaceID == ws.ID {
		s.workspaceID = ""
	}
	u.mu.Unlock()
	return "deleted: " + ws.Name
}

func (u *LgUsecase) handleWorkspaceUse(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	nameOrID := strings.TrimSpace(args)
	if nameOrID == "" {
		return "error: workspace name or ID required"
	}
	ws, err := u.tasks.FindWorkspace(ctx, s.projectID, nameOrID)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	u.mu.Lock()
	s.workspaceID = ws.ID
	u.mu.Unlock()
	return "using workspace: " + ws.Name
}

func (u *LgUsecase) handleCheckpoint(ctx context.Context, s *activeSession, args string) string {
	if u.tasks == nil {
		return "error: task writer not configured"
	}
	if s.workspaceID == "" {
		return "error: no workspace active -- call #lg.workspace.create <name> or #lg.workspace.use <name> first"
	}
	if strings.TrimSpace(args) == "" {
		return `usage: #lg.tasks.checkpoint {"tasks":[{"title":"...","reason":"...","impl":["directive1","directive2"]}]}`
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
	if s.sessionType == "headless" {
		created, err := u.tasks.CreateTasks(ctx, s.workspaceID, tasks)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return fmt.Sprintf("saved: %d tasks", len(created))
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

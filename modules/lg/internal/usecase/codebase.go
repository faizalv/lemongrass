package usecase

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	lge "github.com/faizalv/lemongrass/modules/lg/entity"
	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
)

const interimNoArg = "no interim -- call #lg.codebase.interim first"
const chunkSize    = 60
const chunkOverlap = 10

type chunkResult struct {
	content   string
	lineStart int
	lineEnd   int
}

func chunkLines(lines []string, baseLineStart int) []chunkResult {
	if len(lines) == 0 {
		return nil
	}
	var chunks []chunkResult
	for offset := 0; offset < len(lines); {
		end := offset + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, chunkResult{
			content:   strings.Join(lines[offset:end], "\n"),
			lineStart: baseLineStart + offset,
			lineEnd:   baseLineStart + end - 1,
		})
		if end == len(lines) {
			break
		}
		offset += chunkSize - chunkOverlap
	}
	return chunks
}

func chunkWithSignature(sig string, lines []string, baseLineStart int) []chunkResult {
	chunks := chunkLines(lines, baseLineStart)
	if sig == "" {
		return chunks
	}
	for i := range chunks {
		chunks[i].content = sig + "\n" + chunks[i].content
	}
	return chunks
}

type goSym struct {
	sig       string
	lineStart int
	lines     []string
}

func extractGoSymbols(content string) []goSym {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", []byte(content), 0)
	if err != nil {
		return nil
	}
	contentLines := strings.Split(content, "\n")
	var syms []goSym
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		start := fset.Position(gd.Pos()).Line
		end := fset.Position(gd.End()).Line
		if start < 1 || end > len(contentLines) {
			continue
		}
		syms = append(syms, goSym{
			sig:       "",
			lineStart: start,
			lines:     contentLines[start-1 : end],
		})
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		if start < 1 || end > len(contentLines) || start > end {
			continue
		}
		// sig is the signature line; body is everything after it.
		// chunkWithSignature prepends sig to every chunk, so we exclude
		// the sig line from lines to avoid printing it twice.
		sig := contentLines[start-1]
		body := contentLines[start:end] // 0-indexed: start = line after sig
		syms = append(syms, goSym{
			sig:       sig,
			lineStart: start + 1, // 1-indexed: first body line
			lines:     body,
		})
	}
	return syms
}

func signatureFromNode(n reconentity.SemanticNode) string {
	if n.Receiver != "" && n.Signature != "" {
		return fmt.Sprintf("func (%s) %s%s", n.Receiver, n.Symbol, n.Signature)
	}
	if n.Signature != "" {
		return fmt.Sprintf("func %s%s", n.Symbol, n.Signature)
	}
	if n.Symbol != "" {
		return n.Symbol + " " + n.Kind
	}
	return n.Kind
}

func formatChunks(chunks []lge.InterimChunk) string {
	if len(chunks) == 0 {
		return "no results"
	}
	var sb strings.Builder
	for i, c := range chunks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "%s L%d-%d\n%s", c.FilePath, c.LineStart, c.LineEnd, c.Content)
	}
	return sb.String()
}

func (u *LgUsecase) handleCodebaseInterim(ctx context.Context, sessionID string, s *activeSession, args string) string {
	if u.interim == nil {
		return "error: interim store not available"
	}

	u.interim.DropInterim(ctx, s.key)

	projectDir := ""
	if u.recon != nil {
		rawPath, err := u.recon.ProjectDir(ctx, s.projectID)
		if err == nil {
			projectDir = filepath.Join("/projects", filepath.Base(rawPath))
		}
	}

	type fileJob struct {
		filePath string
		chunks   []chunkResult
	}

	inputs := strings.Split(args, "|")
	var jobs []fileJob

	for _, raw := range inputs {
		input := strings.TrimSpace(raw)
		if input == "" {
			continue
		}
		switch {
		case strings.HasPrefix(input, "S:"):
			rest := input[2:]
			parts := strings.SplitN(rest, ":", 3)
			if len(parts) != 3 {
				continue
			}
			filePath, symbol, kind := parts[0], parts[1], parts[2]
			node, code, err := u.recon.ReadNode(ctx, s.projectID, filePath, symbol, kind)
			if err != nil {
				continue
			}
			sig := signatureFromNode(node)
			codeLines := strings.Split(code, "\n")
			chunks := chunkWithSignature(sig, codeLines, node.LineStart)
			jobs = append(jobs, fileJob{filePath: filePath, chunks: chunks})

		case strings.HasPrefix(input, "F:"):
			if projectDir == "" {
				continue
			}
			relPath := strings.TrimSpace(input[2:])
			diskPath := filepath.Join(projectDir, relPath)
			data, err := os.ReadFile(diskPath)
			if err != nil {
				continue
			}
			var nodes []reconentity.SemanticNode
			if !strings.HasSuffix(relPath, ".go") && u.recon != nil {
				nodes, _ = u.recon.ListFileNodes(ctx, s.projectID, relPath)
			}
			jobs = append(jobs, fileJob{
				filePath: relPath,
				chunks:   chunksForFile(relPath, string(data), nodes),
			})

		case strings.HasPrefix(input, "R:"):
			if projectDir == "" {
				continue
			}
			pattern := strings.TrimSpace(input[2:])
			matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
			if err != nil {
				continue
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				rel, err := filepath.Rel(projectDir, match)
				if err != nil {
					continue
				}
				data, err := os.ReadFile(match)
				if err != nil {
					continue
				}
				var nodes []reconentity.SemanticNode
				if !strings.HasSuffix(rel, ".go") && u.recon != nil {
					nodes, _ = u.recon.ListFileNodes(ctx, s.projectID, rel)
				}
				jobs = append(jobs, fileJob{
					filePath: rel,
					chunks:   chunksForFile(rel, string(data), nodes),
				})
			}
		}
	}

	if len(jobs) == 0 {
		return "error: no files resolved from inputs"
	}

	var wg sync.WaitGroup
	var totalChunks atomic.Int64

	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			totalChunks.Add(int64(len(job.chunks)))
			for i, chunk := range job.chunks {
				var vec []float32
				if u.recon != nil {
					vec, _ = u.recon.Embed(ctx, chunk.content)
				}
				u.interim.InsertChunk(ctx, s.key, job.filePath, i, chunk.lineStart, chunk.lineEnd, chunk.content, vec)
			}
		}()
	}

	wg.Wait()
	return fmt.Sprintf("interim ready: %d files, %d chunks", len(jobs), totalChunks.Load())
}

func chunksForNodes(contentLines []string, nodes []reconentity.SemanticNode) []chunkResult {
	var chunks []chunkResult
	for _, n := range nodes {
		if n.LineStart < 1 || n.LineEnd < n.LineStart || n.LineEnd > len(contentLines) {
			continue
		}
		sig := contentLines[n.LineStart-1]
		body := contentLines[n.LineStart:n.LineEnd] // body without sig line
		if len(body) == 0 {
			chunks = append(chunks, chunkResult{content: sig, lineStart: n.LineStart, lineEnd: n.LineStart})
			continue
		}
		chunks = append(chunks, chunkWithSignature(sig, body, n.LineStart+1)...)
	}
	return chunks
}

func chunksForFile(filePath, content string, nodes []reconentity.SemanticNode) []chunkResult {
	if strings.HasSuffix(filePath, ".go") {
		syms := extractGoSymbols(content)
		if len(syms) > 0 {
			var chunks []chunkResult
			for _, sym := range syms {
				if len(sym.lines) == 0 {
					chunks = append(chunks, chunkResult{content: sym.sig, lineStart: sym.lineStart - 1, lineEnd: sym.lineStart - 1})
					continue
				}
				chunks = append(chunks, chunkWithSignature(sym.sig, sym.lines, sym.lineStart)...)
			}
			return chunks
		}
	}
	if len(nodes) > 0 {
		if c := chunksForNodes(strings.Split(content, "\n"), nodes); len(c) > 0 {
			return c
		}
	}
	return chunkLines(strings.Split(content, "\n"), 1)
}

func (u *LgUsecase) handleCodebaseQuery(ctx context.Context, sessionID string, s *activeSession, args string) string {
	if u.interim == nil {
		return interimNoArg
	}
	has, err := u.interim.HasInterim(ctx, s.key)
	if err != nil || !has {
		return interimNoArg
	}
	if u.recon == nil {
		return "error: recon not available"
	}
	vec, err := u.recon.Embed(ctx, args)
	if err != nil {
		return "error: embed unavailable -- use codebase.search instead"
	}
	chunks, err := u.interim.QueryInterim(ctx, s.key, vec, 5)
	if err != nil {
		return "error: query failed"
	}
	return formatChunks(chunks)
}

func (u *LgUsecase) handleCodebaseSearch(ctx context.Context, _ string, s *activeSession, args string) string {
	if u.recon == nil {
		return "error: recon not available"
	}
	rawPath, err := u.recon.ProjectDir(ctx, s.projectID)
	if err != nil {
		return "error: project path unavailable"
	}
	projectDir := filepath.Join("/projects", filepath.Base(rawPath))

	pattern := strings.TrimSpace(args)
	if pattern == "" {
		return "error: pattern required"
	}

	var pathScope string
	if i := strings.LastIndex(pattern, " "); i >= 0 {
		last := pattern[i+1:]
		if strings.Contains(last, "/") {
			pathScope = last
			pattern = strings.TrimSpace(pattern[:i])
		}
	}

	filePaths := u.recon.ListFilePaths(ctx, s.projectID)

	if pathScope != "" {
		var scoped []string
		for _, p := range filePaths {
			if strings.HasPrefix(p, pathScope) {
				scoped = append(scoped, p)
			}
		}
		if len(scoped) == 0 {
			return fmt.Sprintf("no results for path %q -- if this is part of your pattern, remove the space or escape the slash", pathScope)
		}
		filePaths = scoped
	}

	re, _ := regexp.Compile("(?i)" + pattern)
	matchLine := func(line string) bool {
		if re != nil {
			return re.MatchString(line)
		}
		return strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
	}

	const contextLines = 2
	const maxResults = 50


	type match struct {
		filePath  string
		lineStart int
		lineEnd   int
		content   string
	}

	var results []match
	limitReached := false

	// Path and directory matching -- lgignore-filtered list, no disk access
	dirSeen := make(map[string]bool)
	for _, p := range filePaths {
		if matchLine(p) {
			results = append(results, match{filePath: p, content: "[path]"})
			if len(results) >= maxResults {
				limitReached = true
				break
			}
		}
		for d := filepath.Dir(p); d != "." && d != ""; d = filepath.Dir(d) {
			if dirSeen[d] {
				break
			}
			dirSeen[d] = true
		}
	}
	if !limitReached {
		dirs := make([]string, 0, len(dirSeen))
		for d := range dirSeen {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			if matchLine(d) {
				results = append(results, match{filePath: d + "/", content: "[dir]"})
				if len(results) >= maxResults {
					limitReached = true
					break
				}
			}
		}
	}

	// Content matching -- iterate lgignore-filtered file list
	if !limitReached {
		for _, relPath := range filePaths {
			if limitReached {
				break
			}
			absPath := filepath.Join(projectDir, relPath)
			data, err := os.ReadFile(absPath)
			if err != nil {
				continue
			}
			if bytes.IndexByte(data, 0) >= 0 {
				continue
			}
			fileLines := strings.Split(string(data), "\n")
			for i, line := range fileLines {
				if matchLine(line) {
					start := i - contextLines
					if start < 0 {
						start = 0
					}
					end := i + contextLines
					if end >= len(fileLines) {
						end = len(fileLines) - 1
					}
					results = append(results, match{
						filePath:  relPath,
						lineStart: start + 1,
						lineEnd:   end + 1,
						content:   strings.Join(fileLines[start:end+1], "\n"),
					})
					if len(results) >= maxResults {
						limitReached = true
						break
					}
				}
			}
		}
	}
	if len(results) == 0 {
		return "no results"
	}

	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if r.lineStart == 0 {
			fmt.Fprintf(&sb, "%s  %s", r.filePath, r.content)
		} else {
			fmt.Fprintf(&sb, "%s L%d-%d\n%s", r.filePath, r.lineStart, r.lineEnd, r.content)
		}
	}
	if limitReached {
		fmt.Fprintf(&sb, "\n\n!! capped at %d results -- use a more specific pattern", maxResults)
	}
	return sb.String()
}

package usecase

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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

	pattern := strings.ToLower(strings.TrimSpace(args))
	if pattern == "" {
		return "error: pattern required"
	}

	const contextLines = 3
	const maxResults = 20

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, ".lemongrass": true,
	}

	type match struct {
		filePath  string
		lineStart int
		lineEnd   int
		content   string
	}

	var results []match
	limitReached := false

	_ = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || limitReached {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), pattern) {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines
				if end >= len(lines) {
					end = len(lines) - 1
				}
				results = append(results, match{
					filePath:  rel,
					lineStart: start + 1,
					lineEnd:   end + 1,
					content:   strings.Join(lines[start:end+1], "\n"),
				})
				if len(results) >= maxResults {
					limitReached = true
					return nil
				}
			}
		}
		return nil
	})

	if len(results) == 0 {
		return "no results"
	}

	var sb strings.Builder
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "%s L%d-%d\n%s", r.filePath, r.lineStart, r.lineEnd, r.content)
	}
	if limitReached {
		fmt.Fprintf(&sb, "\n\n(results capped at %d -- refine pattern)", maxResults)
	}
	return sb.String()
}

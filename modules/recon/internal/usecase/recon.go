package usecase

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/infra"
	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/embed"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
)

type repo interface {
	ProjectDir(ctx context.Context, projectID int64) (string, error)
	HasNodes(ctx context.Context, projectID int64) (bool, error)
	UpsertNodes(ctx context.Context, nodes []entity.SemanticNode) error
	MarkRemoved(ctx context.Context, projectID int64, parsedPaths []string, ignoredExisting []string) error
	DeleteByProject(ctx context.Context, projectID int64) error
	ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error)
	GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error)
	GetFileHashes(ctx context.Context, projectID int64) (map[string]string, error)
	UpsertFileHashes(ctx context.Context, projectID int64, hashes []entity.FileHash) error
	DeleteFileHashes(ctx context.Context, projectID int64, paths []string) error
	GetSyncInterval(ctx context.Context, projectID int64) (string, error)
	UpdateSyncInterval(ctx context.Context, projectID int64, interval string) error
	GetNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (entity.SemanticNode, error)
	AnnotateNode(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) (int64, error)
	ListByPathDirect(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, []entity.SubdirSummary, error)
	ListAllNodesByPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, error)
	SetEmbedding(ctx context.Context, projectID int64, filePath, symbol string, embedding []float32) error
	GetTreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]entity.DirectoryCoverage, error)
	ListUnembedded(ctx context.Context, limit int) ([]entity.SemanticNode, error)
	SearchByVector(ctx context.Context, projectID int64, embedding []float32, limit int) ([]entity.SemanticNode, error)
	SearchByFTS(ctx context.Context, projectID int64, query string, limit int) ([]entity.SemanticNode, error)
	GetRelated(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []entity.SemanticNode, err error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	GetLastSyncedCommit(ctx context.Context, projectID int64) (string, error)
	SetLastSyncedCommit(ctx context.Context, projectID int64, commit string) error
	DeleteNodesByFilePaths(ctx context.Context, projectID int64, filePaths []string) error
	GetEmbedPending(ctx context.Context, projectID int64) (pending, total int, err error)
	GetStaleCount(ctx context.Context, projectID int64) (int, error)
	SaveKnowledge(ctx context.Context, projectID int64, key, content string, embedding []float32) error
	ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error)
	SearchKnowledge(ctx context.Context, projectID int64, embedding []float32, limit int) ([]entity.KnowledgeEntry, error)
	ListKnowledge(ctx context.Context, projectID int64) ([]entity.KnowledgeEntry, error)
	DeleteKnowledge(ctx context.Context, projectID int64, key string) (bool, error)
	FindSimilarKnowledge(ctx context.Context, projectID int64, excludeKey string, embedding []float32) ([]string, error)
	DeleteKnowledgeByProject(ctx context.Context, projectID int64) error
}

type ReconUsecase struct {
	parsers    []lang.Parser
	repo       repo
	embed      *embed.Client
	syncMu     sync.Mutex
	syncing    map[int64]bool
	lastSynced map[int64]int64
	activeID   int64

	embedMu      sync.Mutex
	embedCurrent string
	embedRecent  []string
	embedLog     *log.Logger
	embedLogW    io.Closer
}

func New(r repo, logDir string, parsers ...lang.Parser) *ReconUsecase {
	sorted := make([]lang.Parser, len(parsers))
	copy(sorted, parsers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})
	uc := &ReconUsecase{
		parsers:    sorted,
		repo:       r,
		embed:      embed.New(),
		syncing:    make(map[int64]bool),
		lastSynced: make(map[int64]int64),
	}
	if logDir != "" {
		os.MkdirAll(logDir, 0755)
		w := infra.NewDailyRotateWriter(logDir, "embed", 7)
		uc.embedLog = log.New(io.MultiWriter(os.Stderr, w), "[embed] ", log.LstdFlags)
		uc.embedLogW = w
	}
	return uc
}

func (u *ReconUsecase) Close() {
	if u.embedLogW != nil {
		u.embedLogW.Close()
	}
}

func (u *ReconUsecase) MapIfNeeded(ctx context.Context, projectID int64, dir string) error {
	has, err := u.repo.HasNodes(ctx, projectID)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return u.Map(ctx, projectID, dir, nil)
}

func (u *ReconUsecase) Map(ctx context.Context, projectID int64, dir string, ignoredExisting []string) error {
	results, err := u.Build(dir)
	if err != nil {
		return err
	}
	nodes := u.NodesToInsert(projectID, results)
	if err := u.repo.UpsertNodes(ctx, nodes); err != nil {
		return err
	}
	return u.repo.MarkRemoved(ctx, projectID, u.ActiveFilePaths(results), ignoredExisting)
}

func (u *ReconUsecase) Build(dir string) ([]*entity.ParseResult, error) {
	ig := loadIgnore(dir)
	var results []*entity.ParseResult
	for _, p := range u.parsers {
		if !p.Detect(dir) {
			continue
		}
		result, err := p.Parse(dir, ig)
		if err != nil {
			log.Printf("[recon] parser %s: %v", p.Name(), err)
			continue
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no supported language detected in %s", dir)
	}
	return results, nil
}

func (u *ReconUsecase) NodesToInsert(projectID int64, results []*entity.ParseResult) []entity.SemanticNode {
	var nodes []entity.SemanticNode
	for _, result := range results {
		for _, group := range result.Groups {
			for _, n := range group.Nodes {
				nodes = append(nodes, entity.SemanticNode{
					ProjectID:   projectID,
					FilePath:    n.FilePath,
					LineStart:   n.LineStart,
					LineEnd:     n.LineEnd,
					Package:     n.Package,
					Symbol:      n.Symbol,
					Kind:        n.Kind,
					Language:    group.Language,
					Receiver:    n.Receiver,
					Signature:   n.Signature,
					Exported:    n.Exported,
					DependsOn:   n.DependsOn,
					Status:      "unexplored",
					ContentHash: n.ContentHash,
				})
			}
		}
	}
	return nodes
}

func signatureText(n entity.SemanticNode) string {
	role := entity.KindRole(n.Kind)
	if role == "meta" {
		return ""
	}
	switch role {
	case "method":
		sym := n.Symbol
		if n.Receiver != "" {
			sym = "(" + n.Receiver + ") " + n.Symbol
		}
		if n.Signature != "" {
			return sym + n.Signature
		}
		return sym + " " + n.Kind
	case "func":
		if n.Signature != "" {
			return n.Symbol + n.Signature
		}
		return n.Symbol + " " + n.Kind
	default:
		if n.Symbol != "" {
			return n.Symbol + " " + n.Kind
		}
		return n.Kind
	}
}

func (u *ReconUsecase) StartBackgroundEmbed(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			nodes, err := u.repo.ListUnembedded(ctx, 50)
			if err != nil {
				log.Printf("[recon/embed] list unembedded: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}
				continue
			}

			if len(nodes) == 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}
				continue
			}

			embedErr := false
			for _, n := range nodes {
				select {
				case <-ctx.Done():
					return
				default:
				}
				text := signatureText(n)
				if text == "" {
					continue
				}
				key := n.FilePath + ":" + n.Symbol + ":" + n.Kind
				u.embedMu.Lock()
				u.embedCurrent = key
				u.embedMu.Unlock()

				t0 := time.Now()
				vec, err := u.embed.Embed(ctx, text)
				elapsed := time.Since(t0)
				if err != nil {
					u.embedMu.Lock()
					u.embedCurrent = ""
					u.embedMu.Unlock()
					if u.embedLog != nil {
						u.embedLog.Printf("service unavailable: %v -- retrying in 30s", err)
					}
					embedErr = true
					break
				}
				if err := u.repo.SetEmbedding(ctx, n.ProjectID, n.FilePath, n.Symbol, vec); err != nil {
					if u.embedLog != nil {
						u.embedLog.Printf("set %s -- error: %v", key, err)
					}
				} else {
					if u.embedLog != nil {
						u.embedLog.Printf("ok %s (%dms)", key, elapsed.Milliseconds())
					}
					u.embedMu.Lock()
					if len(u.embedRecent) >= 20 {
						u.embedRecent = u.embedRecent[1:]
					}
					u.embedRecent = append(u.embedRecent, key)
					u.embedMu.Unlock()
				}
				select {
				case <-ctx.Done():
					u.embedMu.Lock()
					u.embedCurrent = ""
					u.embedMu.Unlock()
					return
				case <-time.After(50 * time.Millisecond):
				}
			}
			u.embedMu.Lock()
			u.embedCurrent = ""
			u.embedMu.Unlock()
			if embedErr {
				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}
				continue
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()
}

func (u *ReconUsecase) ActiveFilePaths(results []*entity.ParseResult) []string {
	seen := make(map[string]bool)
	for _, result := range results {
		for _, group := range result.Groups {
			for _, n := range group.Nodes {
				seen[n.FilePath] = true
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}

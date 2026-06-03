package usecase

import (
	"context"
	"fmt"
	"sort"
	"sync"

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
	AnnotateNode(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) error
	ListByPathPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, error)
	SetEmbedding(ctx context.Context, projectID int64, filePath, symbol string, embedding []float32) error
	GetTreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]entity.DirectoryCoverage, error)
	SearchByVector(ctx context.Context, projectID int64, embedding []float32, limit int) ([]entity.SemanticNode, error)
	SearchByFTS(ctx context.Context, projectID int64, query string, limit int) ([]entity.SemanticNode, error)
	GetRelated(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []entity.SemanticNode, err error)
	GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error)
	GetLastSyncedCommit(ctx context.Context, projectID int64) (string, error)
	SetLastSyncedCommit(ctx context.Context, projectID int64, commit string) error
	DeleteNodesByFilePaths(ctx context.Context, projectID int64, filePaths []string) error
	GetStaleCount(ctx context.Context, projectID int64) (int, error)
}

type ReconUsecase struct {
	parsers    []lang.Parser
	repo       repo
	embed      *embed.Client
	syncMu     sync.Mutex
	syncing    map[int64]bool
	lastSynced map[int64]int64
	activeID   int64
}

func New(r repo, parsers ...lang.Parser) *ReconUsecase {
	sorted := make([]lang.Parser, len(parsers))
	copy(sorted, parsers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})
	return &ReconUsecase{
		parsers:    sorted,
		repo:       r,
		embed:      embed.New(),
		syncing:    make(map[int64]bool),
		lastSynced: make(map[int64]int64),
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
	trees, err := u.Build(dir)
	if err != nil {
		return err
	}
	nodes := u.NodesToInsert(projectID, trees)
	if err := u.repo.UpsertNodes(ctx, nodes); err != nil {
		return err
	}
	return u.repo.MarkRemoved(ctx, projectID, u.ActiveFilePaths(trees), ignoredExisting)
}

func (u *ReconUsecase) Build(dir string) ([]*entity.ProjectTree, error) {
	ig := loadIgnore(dir)
	var trees []*entity.ProjectTree
	for _, p := range u.parsers {
		if p.Detect(dir) {
			tree, err := p.Parse(dir, ig)
			if err != nil {
				return nil, fmt.Errorf("parser %s: %w", p.Name(), err)
			}
			trees = append(trees, tree)
		}
	}
	if len(trees) == 0 {
		return nil, fmt.Errorf("no supported language detected in %s", dir)
	}
	return trees, nil
}

func (u *ReconUsecase) NodesToInsert(projectID int64, trees []*entity.ProjectTree) []entity.SemanticNode {
	var nodes []entity.SemanticNode
	for _, tree := range trees {
		for _, pkg := range tree.Packages {
			for _, file := range pkg.Files {
				for _, sym := range file.Exports {
					nodes = append(nodes, entity.SemanticNode{
						ProjectID:   projectID,
						FilePath:    file.Path,
						LineStart:   sym.LineStart,
						LineEnd:     sym.LineEnd,
						Package:     pkg.ImportPath,
						Symbol:      sym.Name,
						Kind:        sym.Kind,
						Language:    tree.Language,
						Receiver:    sym.Receiver,
						Signature:   sym.Signature,
						Exported:    true,
						DependsOn:   pkg.DependsOn,
						Status:      "unexplored",
						ContentHash: sym.ContentHash,
					})
				}
			}
		}
	}
	return nodes
}

func (u *ReconUsecase) ActiveFilePaths(trees []*entity.ProjectTree) []string {
	seen := make(map[string]bool)
	for _, tree := range trees {
		for _, pkg := range tree.Packages {
			for _, file := range pkg.Files {
				seen[file.Path] = true
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}

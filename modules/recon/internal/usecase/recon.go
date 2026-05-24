package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/modules/recon/entity"
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
}

type ReconUsecase struct {
	parsers    []lang.Parser
	repo       repo
	syncMu     sync.Mutex
	syncing    map[int64]bool
	lastSynced map[int64]int64 // unix nano
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
		syncing:    make(map[int64]bool),
		lastSynced: make(map[int64]int64),
	}
}

// MapIfNeeded maps the project if it has no nodes yet. Cold-start path only.
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

// Map (re)maps a project unconditionally. ignoredExisting exempts present-but-ignored
// files from being marked removed — pass nil when calling outside of Sync.
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

func (u *ReconUsecase) ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error) {
	return u.repo.ListNodes(ctx, projectID, language, kind, status)
}

func (u *ReconUsecase) GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error) {
	return u.repo.GetCoverage(ctx, projectID)
}

// ── Sync state ────────────────────────────────────────────────────────────────

func (u *ReconUsecase) Activate(projectID int64) {
	u.syncMu.Lock()
	u.activeID = projectID
	if u.syncing[projectID] {
		u.syncMu.Unlock()
		return
	}
	u.syncing[projectID] = true
	u.syncMu.Unlock()

	go func() {
		defer func() {
			u.syncMu.Lock()
			u.syncing[projectID] = false
			u.lastSynced[projectID] = time.Now().UnixNano()
			u.syncMu.Unlock()
		}()
		rawPath, err := u.repo.ProjectDir(context.Background(), projectID)
		if err != nil {
			return
		}
		dir := "/projects/" + filepath.Base(rawPath)
		u.Sync(context.Background(), projectID, dir)
	}()
}

func (u *ReconUsecase) SyncStatus(projectID int64) (syncing bool, lastSyncedNano int64) {
	u.syncMu.Lock()
	defer u.syncMu.Unlock()
	return u.syncing[projectID], u.lastSynced[projectID]
}

func (u *ReconUsecase) TickScheduler(ctx context.Context) {
	u.syncMu.Lock()
	activeID := u.activeID
	u.syncMu.Unlock()
	if activeID == 0 {
		return
	}
	interval, err := u.repo.GetSyncInterval(ctx, activeID)
	if err != nil || interval == "off" {
		return
	}
	dur := intervalDuration(interval)
	if dur == 0 {
		return
	}
	u.syncMu.Lock()
	lastNano := u.lastSynced[activeID]
	u.syncMu.Unlock()
	if lastNano > 0 && time.Since(time.Unix(0, lastNano)) < dur {
		return
	}
	u.Activate(activeID)
}

func (u *ReconUsecase) UpdateSyncInterval(ctx context.Context, projectID int64, interval string) error {
	return u.repo.UpdateSyncInterval(ctx, projectID, interval)
}

func (u *ReconUsecase) GetSyncInterval(ctx context.Context, projectID int64) (string, error) {
	return u.repo.GetSyncInterval(ctx, projectID)
}

func intervalDuration(s string) time.Duration {
	switch s {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	}
	return 0
}

// ── LgIgnore ──────────────────────────────────────────────────────────────────

func (u *ReconUsecase) GetLgIgnorePatterns(ctx context.Context, projectID int64) ([]string, error) {
	rawPath, err := u.repo.ProjectDir(ctx, projectID)
	if err != nil {
		return nil, err
	}
	dir := "/projects/" + filepath.Base(rawPath)
	patterns := readUserPatterns(dir)
	if patterns == nil {
		patterns = []string{}
	}
	return patterns, nil
}

// Build runs all matching parsers against dir and returns one tree per language.
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

// NodesToInsert converts parsed trees into SemanticNode slices ready for DB insertion.
func (u *ReconUsecase) NodesToInsert(projectID int64, trees []*entity.ProjectTree) []entity.SemanticNode {
	var nodes []entity.SemanticNode
	for _, tree := range trees {
		for _, pkg := range tree.Packages {
			for _, file := range pkg.Files {
				for _, sym := range file.Exports {
					nodes = append(nodes, entity.SemanticNode{
						ProjectID: projectID,
						FilePath:  file.Path,
						LineStart: sym.LineStart,
						LineEnd:   sym.LineEnd,
						Package:   pkg.ImportPath,
						Symbol:    sym.Name,
						Kind:      sym.Kind,
						Language:  tree.Language,
						Receiver:  sym.Receiver,
						Signature: sym.Signature,
						Exported:  true,
						DependsOn: pkg.DependsOn,
						Status:    "unexplored",
					})
				}
			}
		}
	}
	return nodes
}

// ActiveFilePaths returns the set of file paths present in the parsed trees.
// Used to detect removed files during re-mapping.
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

// Format renders a ProjectTree as compact structured text for model consumption.
func (u *ReconUsecase) Format(tree *entity.ProjectTree) string {
	var sb strings.Builder
	sb.WriteString("module " + tree.Module + "\n\n")

	pkgs := sortedPackages(tree.Packages)
	for _, pkg := range pkgs {
		writePackageBlock(&sb, pkg, tree.Module)
	}
	return sb.String()
}

// FormatDeps renders a focused dependency view for the given package dirs.
func (u *ReconUsecase) FormatDeps(tree *entity.ProjectTree, dirs []string) string {
	dirSet := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		dirSet[d] = true
	}

	var sb strings.Builder
	sb.WriteString("module " + tree.Module + "\n\n")

	pkgs := sortedPackages(tree.Packages)
	for _, pkg := range pkgs {
		if dirSet[pkg.Dir] {
			writePackageBlock(&sb, pkg, tree.Module)
		}
	}
	return sb.String()
}

func writePackageBlock(sb *strings.Builder, pkg entity.PackageNode, module string) {
	pkgName := packageName(pkg)
	sb.WriteString(fmt.Sprintf("%s [package %s]\n", pkg.Dir, pkgName))

	if len(pkg.DependsOn) > 0 {
		sb.WriteString("  imports: " + shortPaths(pkg.DependsOn, module) + "\n")
	}

	exports := mergedExports(pkg)
	if len(exports) > 0 {
		sb.WriteString("  exports: " + strings.Join(exports, ", ") + "\n")
	}

	if len(pkg.UsedBy) > 0 {
		sb.WriteString("  used by: " + shortPaths(pkg.UsedBy, module) + "\n")
	}

	sb.WriteString("\n")
}

func packageName(pkg entity.PackageNode) string {
	for _, f := range pkg.Files {
		if f.Package != "" {
			return f.Package
		}
	}
	return "?"
}

func mergedExports(pkg entity.PackageNode) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range pkg.Files {
		for _, sym := range f.Exports {
			key := sym.Name
			if !seen[key] {
				seen[key] = true
				out = append(out, sym.Name+" ("+sym.Kind+")")
			}
		}
	}
	sort.Strings(out)
	return out
}

func shortPaths(paths []string, module string) string {
	short := make([]string, len(paths))
	for i, p := range paths {
		short[i] = strings.TrimPrefix(p, module+"/")
	}
	sort.Strings(short)
	return strings.Join(short, ", ")
}

func sortedPackages(pkgs []entity.PackageNode) []entity.PackageNode {
	out := make([]entity.PackageNode, len(pkgs))
	copy(out, pkgs)
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out
}

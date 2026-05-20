package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
)

type repo interface {
	HasNodes(ctx context.Context, projectID int64) (bool, error)
	UpsertNodes(ctx context.Context, nodes []entity.SemanticNode) error
	MarkRemoved(ctx context.Context, projectID int64, activePaths []string) error
	DeleteByProject(ctx context.Context, projectID int64) error
	ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error)
	GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error)
}

type ReconUsecase struct {
	parsers []lang.Parser
	repo    repo
}

func New(r repo, parsers ...lang.Parser) *ReconUsecase {
	sorted := make([]lang.Parser, len(parsers))
	copy(sorted, parsers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})
	return &ReconUsecase{parsers: sorted, repo: r}
}

// MapIfNeeded maps the project if it has no nodes yet. Safe to call on every startup.
func (u *ReconUsecase) MapIfNeeded(ctx context.Context, projectID int64, dir string) error {
	has, err := u.repo.HasNodes(ctx, projectID)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return u.Map(ctx, projectID, dir)
}

// Map (re)maps a project unconditionally. Preserves existing descriptions and embeddings.
func (u *ReconUsecase) Map(ctx context.Context, projectID int64, dir string) error {
	trees, err := u.Build(dir)
	if err != nil {
		return err
	}
	nodes := u.NodesToInsert(projectID, trees)
	if err := u.repo.UpsertNodes(ctx, nodes); err != nil {
		return err
	}
	return u.repo.MarkRemoved(ctx, projectID, u.ActiveFilePaths(trees))
}

func (u *ReconUsecase) ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error) {
	return u.repo.ListNodes(ctx, projectID, language, kind, status)
}

func (u *ReconUsecase) GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error) {
	return u.repo.GetCoverage(ctx, projectID)
}

// Build runs all matching parsers against dir and returns one tree per language.
func (u *ReconUsecase) Build(dir string) ([]*entity.ProjectTree, error) {
	var trees []*entity.ProjectTree
	for _, p := range u.parsers {
		if p.Detect(dir) {
			tree, err := p.Parse(dir)
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

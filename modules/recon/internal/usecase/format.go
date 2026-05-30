package usecase

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

func (u *ReconUsecase) Format(tree *entity.ProjectTree) string {
	var sb strings.Builder
	sb.WriteString("module " + tree.Module + "\n\n")

	pkgs := sortedPackages(tree.Packages)
	for _, pkg := range pkgs {
		writePackageBlock(&sb, pkg, tree.Module)
	}
	return sb.String()
}

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

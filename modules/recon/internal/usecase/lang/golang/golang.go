package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

type Parser struct{}

func New() *Parser { return &Parser{} }

func (p *Parser) Name() string { return "go" }

func (p *Parser) Detect(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(abs, "go.mod"))
	return err == nil
}

func (p *Parser) Parse(dir string) (*entity.ProjectTree, error) {
	// Resolve to absolute so WalkDir names and Rel calculations are clean.
	// A relative path like "../../" has Base == ".." which would trigger shouldSkip.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = abs

	moduleName, err := readModuleName(dir)
	if err != nil {
		return nil, err
	}

	var packages []entity.PackageNode
	importIndex := make(map[string]int) // importPath -> index in packages

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if shouldSkip(d.Name()) {
			return filepath.SkipDir
		}
		pkg := parseDir(path, dir, moduleName)
		if pkg == nil {
			return nil
		}
		importIndex[pkg.ImportPath] = len(packages)
		packages = append(packages, *pkg)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	// Second pass: build UsedBy from DependsOn
	for i := range packages {
		for _, dep := range packages[i].DependsOn {
			if j, ok := importIndex[dep]; ok {
				packages[j].UsedBy = append(packages[j].UsedBy, packages[i].ImportPath)
			}
		}
	}

	return &entity.ProjectTree{Module: moduleName, Root: dir, Packages: packages}, nil
}

func parseDir(dir, root, moduleName string) *entity.PackageNode {
	fset := token.NewFileSet()
	// Skip test files — they add noise without changing the public API shape.
	pkgMap, _ := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if len(pkgMap) == 0 {
		return nil
	}

	// Pick the non-test package when there are multiple (e.g. package + package_test).
	var astPkg *ast.Package
	for name, p := range pkgMap {
		if !strings.HasSuffix(name, "_test") {
			astPkg = p
			break
		}
	}
	if astPkg == nil {
		return nil
	}

	rel, _ := filepath.Rel(root, dir)
	rel = filepath.ToSlash(rel)
	importPath := moduleName
	if rel != "." {
		importPath = moduleName + "/" + rel
	}

	importSet := make(map[string]bool)
	var files []entity.FileNode

	for filePath, astFile := range astPkg.Files {
		relFile, _ := filepath.Rel(root, filePath)
		node := entity.FileNode{
			Path:    filepath.ToSlash(relFile),
			Package: astPkg.Name,
			Exports: extractExports(astFile),
		}
		for _, imp := range astFile.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			node.Imports = append(node.Imports, path)
			importSet[path] = true
		}
		sort.Strings(node.Imports)
		files = append(files, node)
	}

	// Sort files for stable output
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	// DependsOn: only internal imports (same module)
	var dependsOn []string
	for imp := range importSet {
		if strings.HasPrefix(imp, moduleName+"/") || imp == moduleName {
			dependsOn = append(dependsOn, imp)
		}
	}
	sort.Strings(dependsOn)

	return &entity.PackageNode{
		ImportPath: importPath,
		Dir:        rel,
		Files:      files,
		DependsOn:  dependsOn,
	}
}

func extractExports(f *ast.File) []entity.Symbol {
	var symbols []entity.Symbol
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() && d.Recv == nil {
				symbols = append(symbols, entity.Symbol{Name: d.Name.Name, Kind: "func"})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					kind := "type"
					if _, ok := s.Type.(*ast.InterfaceType); ok {
						kind = "interface"
					}
					symbols = append(symbols, entity.Symbol{Name: s.Name.Name, Kind: kind})
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							kind := "var"
							if d.Tok == token.CONST {
								kind = "const"
							}
							symbols = append(symbols, entity.Symbol{Name: name.Name, Kind: kind})
						}
					}
				}
			}
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].Name < symbols[j].Name })
	return symbols
}

func readModuleName(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

func shouldSkip(name string) bool {
	switch name {
	case "vendor", "testdata", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".")
}

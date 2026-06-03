package golang

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang"
)

type Parser struct{}

func New() *Parser { return &Parser{} }

func (p *Parser) Name() string     { return "go" }
func (p *Parser) Priority() int    { return 80 }

func (p *Parser) Detect(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(abs, "go.mod"))
	return err == nil
}

func (p *Parser) ParseFiles(dir string, ig lang.Ignorer, paths []string) (*entity.ProjectTree, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = abs

	moduleName, err := readModuleName(dir)
	if err != nil {
		return nil, err
	}

	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[filepath.ToSlash(p)] = true
	}

	dirSet := make(map[string]bool)
	for _, p := range paths {
		dirSet[filepath.ToSlash(filepath.Dir(p))] = true
	}

	var packages []entity.PackageNode
	for relDir := range dirSet {
		absDir := filepath.Join(dir, relDir)
		pkg := parseDir(absDir, dir, moduleName)
		if pkg == nil {
			continue
		}
		var filtered []entity.FileNode
		for _, f := range pkg.Files {
			if pathSet[f.Path] {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		pkg.Files = filtered
		packages = append(packages, *pkg)
	}

	return &entity.ProjectTree{Language: "go", Module: moduleName, Root: dir, Packages: packages}, nil
}

func (p *Parser) Parse(dir string, ig lang.Ignorer) (*entity.ProjectTree, error) {
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
	importIndex := make(map[string]int)

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if rel != "." && ig.Match(rel+"/") {
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

	for i := range packages {
		for _, dep := range packages[i].DependsOn {
			if j, ok := importIndex[dep]; ok {
				packages[j].UsedBy = append(packages[j].UsedBy, packages[i].ImportPath)
			}
		}
	}

	return &entity.ProjectTree{Language: "go", Module: moduleName, Root: dir, Packages: packages}, nil
}

func parseDir(dir, root, moduleName string) *entity.PackageNode {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()

	type parsedFile struct {
		path    string
		pkgName string
		astFile *ast.File
	}
	var parsed []parsedFile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filePath := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			continue
		}
		pkgName := f.Name.Name
		if strings.HasSuffix(pkgName, "_test") {
			continue
		}
		parsed = append(parsed, parsedFile{path: filePath, pkgName: pkgName, astFile: f})
	}

	if len(parsed) == 0 {
		return nil
	}

	pkgName := parsed[0].pkgName

	rel, _ := filepath.Rel(root, dir)
	rel = filepath.ToSlash(rel)
	importPath := moduleName
	if rel != "." {
		importPath = moduleName + "/" + rel
	}

	importSet := make(map[string]bool)
	var files []entity.FileNode

	for _, pf := range parsed {
		relFile, _ := filepath.Rel(root, pf.path)
		src, _ := os.ReadFile(pf.path)
		srcLines := bytes.Split(src, []byte("\n"))
		exports := extractExports(fset, pf.astFile, srcLines)
		for _, decl := range pf.astFile.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.IMPORT || len(genDecl.Specs) == 0 {
				continue
			}
			ls := fset.Position(genDecl.Pos()).Line
			le := fset.Position(genDecl.End()).Line
			exports = append(exports, entity.Symbol{
				Name:        "imports",
				Kind:        "imports",
				LineStart:   ls,
				LineEnd:     le,
				ContentHash: hashLines(srcLines, ls, le),
			})
			break
		}
		node := entity.FileNode{
			Path:    filepath.ToSlash(relFile),
			Package: pkgName,
			Exports: exports,
		}
		for _, imp := range pf.astFile.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			node.Imports = append(node.Imports, path)
			importSet[path] = true
		}
		sort.Strings(node.Imports)
		files = append(files, node)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

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

func hashLines(lines [][]byte, start, end int) string {
	if start < 1 || end < start || end > len(lines) {
		return ""
	}
	h := sha256.New()
	for _, line := range lines[start-1 : end] {
		h.Write(line)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func extractExports(fset *token.FileSet, f *ast.File, srcLines [][]byte) []entity.Symbol {
	var symbols []entity.Symbol
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			isSpecial := d.Name.Name == "main" || d.Name.Name == "init"
			if !d.Name.IsExported() && !isSpecial {
				continue
			}
			sym := entity.Symbol{
				Name:      d.Name.Name,
				LineStart: fset.Position(d.Pos()).Line,
				LineEnd:   fset.Position(d.End()).Line,
				Signature: formatParams(fset, d.Type.Params),
			}
			sym.ContentHash = hashLines(srcLines, sym.LineStart, sym.LineEnd)
			if d.Recv != nil {
				sym.Kind = "method"
				sym.Receiver = receiverTypeName(d.Recv)
			} else {
				sym.Kind = "func"
			}
			symbols = append(symbols, sym)

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					kind := "type"
					switch s.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					ls := fset.Position(s.Pos()).Line
					le := fset.Position(s.End()).Line
					symbols = append(symbols, entity.Symbol{
						Name:        s.Name.Name,
						Kind:        kind,
						LineStart:   ls,
						LineEnd:     le,
						ContentHash: hashLines(srcLines, ls, le),
					})
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							kind := "var"
							if d.Tok == token.CONST {
								kind = "const"
							}
							ls := fset.Position(name.Pos()).Line
							le := fset.Position(name.End()).Line
							symbols = append(symbols, entity.Symbol{
								Name:        name.Name,
								Kind:        kind,
								LineStart:   ls,
								LineEnd:     le,
								ContentHash: hashLines(srcLines, ls, le),
							})
						}
					}
				}
			}
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].Name < symbols[j].Name })
	return symbols
}

func formatParams(fset *token.FileSet, params *ast.FieldList) string {
	if params == nil || params.NumFields() == 0 {
		return ""
	}
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, params)
	return buf.String()
}

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
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


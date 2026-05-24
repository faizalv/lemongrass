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
	fset := token.NewFileSet()
	pkgMap, _ := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if len(pkgMap) == 0 {
		return nil
	}

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
		src, _ := os.ReadFile(filePath)
		srcLines := bytes.Split(src, []byte("\n"))
		node := entity.FileNode{
			Path:    filepath.ToSlash(relFile),
			Package: astPkg.Name,
			Exports: extractExports(fset, astFile, srcLines),
		}
		for _, imp := range astFile.Imports {
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
			if !d.Name.IsExported() {
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


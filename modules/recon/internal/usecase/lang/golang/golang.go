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

func (p *Parser) Name() string  { return "go" }
func (p *Parser) Priority() int { return 80 }

func (p *Parser) Detect(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(abs, "go.mod"))
	return err == nil
}

type parsedASTFile struct {
	path      string
	astFile   *ast.File
	importMap map[string]string // local alias -> full import path
}

type parsedPkg struct {
	node     entity.PackageNode
	fset     *token.FileSet
	astFiles []parsedASTFile
}

func (p *Parser) ParseFiles(dir string, ig lang.Ignorer, paths []string) (*entity.ParseResult, error) {
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
		pp := parseDir(absDir, dir, moduleName)
		if pp == nil {
			continue
		}
		var filtered []entity.FileNode
		for _, f := range pp.node.Files {
			if pathSet[f.Path] {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		pp.node.Files = filtered
		packages = append(packages, pp.node)
	}

	tree := &entity.ProjectTree{Language: "go", Module: moduleName, Root: dir, Packages: packages}
	return tree.ToParseResult(), nil
}

func (p *Parser) Parse(dir string, ig lang.Ignorer) (*entity.ParseResult, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = abs

	moduleName, err := readModuleName(dir)
	if err != nil {
		return nil, err
	}

	var parsedPkgs []parsedPkg
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
		pp := parseDir(path, dir, moduleName)
		if pp == nil {
			return nil
		}
		importIndex[pp.node.ImportPath] = len(parsedPkgs)
		parsedPkgs = append(parsedPkgs, *pp)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	for i := range parsedPkgs {
		for _, dep := range parsedPkgs[i].node.DependsOn {
			if j, ok := importIndex[dep]; ok {
				parsedPkgs[j].node.UsedBy = append(parsedPkgs[j].node.UsedBy, parsedPkgs[i].node.ImportPath)
			}
		}
	}

	// Build symbol index: importPath -> set of exported symbol names
	allPkgSyms := make(map[string]map[string]bool, len(parsedPkgs))
	for _, pp := range parsedPkgs {
		syms := make(map[string]bool)
		for _, f := range pp.node.Files {
			for _, s := range f.Exports {
				if s.Kind != "imports" {
					syms[s.Name] = true
				}
			}
		}
		allPkgSyms[pp.node.ImportPath] = syms
	}

	// Second pass: extract calls per FuncDecl using the full symbol index
	for i := range parsedPkgs {
		pp := &parsedPkgs[i]
		samePackageSyms := allPkgSyms[pp.node.ImportPath]
		for _, af := range pp.astFiles {
			fileIdx := -1
			for k, f := range pp.node.Files {
				if f.Path == af.path {
					fileIdx = k
					break
				}
			}
			if fileIdx < 0 {
				continue
			}
			for _, decl := range af.astFile.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				calls := extractCallsForFunc(fn, samePackageSyms, af.importMap, allPkgSyms)
				if len(calls) == 0 {
					continue
				}
				symLineStart := pp.fset.Position(fn.Pos()).Line
				for k, sym := range pp.node.Files[fileIdx].Exports {
					if sym.LineStart == symLineStart {
						pp.node.Files[fileIdx].Exports[k].Calls = calls
						break
					}
				}
			}
		}
	}

	packages := make([]entity.PackageNode, len(parsedPkgs))
	for i, pp := range parsedPkgs {
		packages[i] = pp.node
	}

	tree := &entity.ProjectTree{Language: "go", Module: moduleName, Root: dir, Packages: packages}
	return tree.ToParseResult(), nil
}

func parseDir(dir, root, moduleName string) *parsedPkg {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()

	type rawFile struct {
		path    string
		pkgName string
		astFile *ast.File
	}
	var parsed []rawFile

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
		parsed = append(parsed, rawFile{path: filePath, pkgName: pkgName, astFile: f})
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
	var astFiles []parsedASTFile

	for _, pf := range parsed {
		relFile, _ := filepath.Rel(root, pf.path)
		relFile = filepath.ToSlash(relFile)
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

		importMap := make(map[string]string, len(pf.astFile.Imports))
		for _, imp := range pf.astFile.Imports {
			rawPath := strings.Trim(imp.Path.Value, `"`)
			var alias string
			if imp.Name != nil {
				if imp.Name.Name == "_" || imp.Name.Name == "." {
					continue
				}
				alias = imp.Name.Name
			} else {
				alias = filepath.Base(rawPath)
			}
			importMap[alias] = rawPath
		}

		node := entity.FileNode{
			Path:    relFile,
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

		astFiles = append(astFiles, parsedASTFile{
			path:      relFile,
			astFile:   pf.astFile,
			importMap: importMap,
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	var dependsOn []string
	for imp := range importSet {
		if strings.HasPrefix(imp, moduleName+"/") || imp == moduleName {
			dependsOn = append(dependsOn, imp)
		}
	}
	sort.Strings(dependsOn)

	return &parsedPkg{
		node: entity.PackageNode{
			ImportPath: importPath,
			Dir:        rel,
			Files:      files,
			DependsOn:  dependsOn,
		},
		fset:     fset,
		astFiles: astFiles,
	}
}

func extractCallsForFunc(fn *ast.FuncDecl, samePackageSyms map[string]bool, importAliasToPath map[string]string, allPkgSyms map[string]map[string]bool) []string {
	if fn.Body == nil {
		return nil
	}
	seen := make(map[string]bool)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if samePackageSyms[fun.Name] {
				seen[fun.Name] = true
			}
		case *ast.SelectorExpr:
			alias, ok := fun.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkgPath, hasPkg := importAliasToPath[alias.Name]; hasPkg {
				if pkgSyms, ok := allPkgSyms[pkgPath]; ok && pkgSyms[fun.Sel.Name] {
					seen[fun.Sel.Name] = true
				}
			} else if samePackageSyms[fun.Sel.Name] {
				// method call on a local var/receiver -- record if the name resolves to same-package symbol
				seen[fun.Sel.Name] = true
			}
		}
		return true
	})
	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
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
			sym := entity.Symbol{
				Name:      d.Name.Name,
				LineStart: fset.Position(d.Pos()).Line,
				LineEnd:   fset.Position(d.End()).Line,
				Signature: formatParams(fset, d.Type.Params),
				Exported:  d.Name.IsExported(),
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

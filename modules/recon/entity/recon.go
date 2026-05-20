package entity

type Symbol struct {
	Name string
	Kind string // "func" | "type" | "interface" | "const" | "var"
}

type FileNode struct {
	Path    string
	Package string
	Imports []string
	Exports []Symbol
}

type PackageNode struct {
	ImportPath string
	Dir        string // relative to project root
	Files      []FileNode
	DependsOn  []string // internal import paths this package imports
	UsedBy     []string // internal import paths that import this package
}

type ProjectTree struct {
	Module   string
	Root     string
	Packages []PackageNode
}

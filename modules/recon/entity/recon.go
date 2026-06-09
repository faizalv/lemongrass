package entity

type Symbol struct {
	Name        string
	Kind        string // "func" | "method" | "type" | "struct" | "interface" | "const" | "var"
	LineStart   int
	LineEnd     int
	Receiver    string   // method receiver type name; empty for non-methods
	Signature   string   // parameter list, e.g. "(ctx context.Context, id int64)"
	ContentHash string   // SHA-256 hex of raw source bytes for the symbol body
	Calls       []string // bare symbol names called within this symbol's body
	Exported    bool
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
	Language string
	Module   string
	Root     string
	Packages []PackageNode
}

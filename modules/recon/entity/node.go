package entity

import "time"

type SemanticNode struct {
	ID          string
	ProjectID   int64
	FilePath    string
	LineStart   int
	LineEnd     int
	Package     string
	Symbol      string
	Kind        string
	Language    string
	Receiver    string
	Signature   string
	Exported    bool
	DependsOn   []string
	Status      string
	Description string
	ReturnType  string
	ContentHash string
	ExploredAt  *time.Time
	CreatedAt   time.Time
}

type LangCoverage struct {
	Language string
	Total    int
	Explored int
	Stale    int
}

type FileHash struct {
	Path string
	Hash string
}

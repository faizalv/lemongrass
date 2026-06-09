package entity

type ParseResult struct {
	Groups []LangGroup
}

type LangGroup struct {
	Language string      `json:"language"`
	Nodes    []ParsedNode `json:"nodes"`
}

type ParsedNode struct {
	FilePath    string   `json:"file_path"`
	Package     string   `json:"package"`
	Symbol      string   `json:"symbol"`
	Kind        string   `json:"kind"`
	LineStart   int      `json:"line_start"`
	LineEnd     int      `json:"line_end"`
	Receiver    string   `json:"receiver"`
	Signature   string   `json:"signature"`
	Exported    bool     `json:"exported"`
	DependsOn   []string `json:"depends_on"`
	ContentHash string   `json:"content_hash"`
	Calls       []string `json:"calls"`
}

func (t *ProjectTree) ToParseResult() *ParseResult {
	var nodes []ParsedNode
	for _, pkg := range t.Packages {
		for _, file := range pkg.Files {
			for _, sym := range file.Exports {
				nodes = append(nodes, ParsedNode{
					FilePath:    file.Path,
					Package:     pkg.ImportPath,
					Symbol:      sym.Name,
					Kind:        sym.Kind,
					LineStart:   sym.LineStart,
					LineEnd:     sym.LineEnd,
					Receiver:    sym.Receiver,
					Signature:   sym.Signature,
					Exported:    sym.Exported,
					DependsOn:   pkg.DependsOn,
					ContentHash: sym.ContentHash,
					Calls:       sym.Calls,
				})
			}
		}
	}
	return &ParseResult{
		Groups: []LangGroup{{Language: t.Language, Nodes: nodes}},
	}
}

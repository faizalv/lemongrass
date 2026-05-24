package transporter

import "github.com/faizalv/lemongrass/modules/recon/entity"

type NodeResponse struct {
	ID          string `json:"id"`
	FilePath    string `json:"file_path"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	Package     string `json:"package"`
	Symbol      string `json:"symbol"`
	Kind        string `json:"kind"`
	Language    string `json:"language"`
	Receiver    string `json:"receiver,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

func NodeToResponse(n entity.SemanticNode) NodeResponse {
	return NodeResponse{
		ID:          n.ID,
		FilePath:    n.FilePath,
		LineStart:   n.LineStart,
		LineEnd:     n.LineEnd,
		Package:     n.Package,
		Symbol:      n.Symbol,
		Kind:        n.Kind,
		Language:    n.Language,
		Receiver:    n.Receiver,
		Signature:   n.Signature,
		Status:      n.Status,
		Description: n.Description,
	}
}

type LangCoverageResponse struct {
	Language string `json:"language"`
	Total    int    `json:"total"`
	Explored int    `json:"explored"`
	Stale    int    `json:"stale"`
}

func CoverageToResponse(c entity.LangCoverage) LangCoverageResponse {
	return LangCoverageResponse{
		Language: c.Language,
		Total:    c.Total,
		Explored: c.Explored,
		Stale:    c.Stale,
	}
}

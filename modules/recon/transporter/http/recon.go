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

type ChangedFileResponse struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type CommitInfoResponse struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
}

type GitStatusResponse struct {
	IsGitRepo     bool                  `json:"is_git_repo"`
	Branch        string                `json:"branch,omitempty"`
	HeadCommit    string                `json:"head_commit,omitempty"`
	HeadMessage   string                `json:"head_message,omitempty"`
	ChangedFiles  []ChangedFileResponse `json:"changed_files,omitempty"`
	StaleCount    int                   `json:"stale_count"`
	RecentCommits []CommitInfoResponse  `json:"recent_commits,omitempty"`
}

func GitStatusToResponse(s entity.GitStatus) GitStatusResponse {
	resp := GitStatusResponse{
		IsGitRepo:  s.IsGitRepo,
		StaleCount: s.StaleCount,
	}
	if !s.IsGitRepo {
		return resp
	}
	resp.Branch = s.Branch
	resp.HeadCommit = s.HeadCommit
	resp.HeadMessage = s.HeadMessage
	resp.ChangedFiles = make([]ChangedFileResponse, len(s.ChangedFiles))
	for i, f := range s.ChangedFiles {
		resp.ChangedFiles[i] = ChangedFileResponse{Path: f.Path, Status: f.Status}
	}
	resp.RecentCommits = make([]CommitInfoResponse, len(s.RecentCommits))
	for i, c := range s.RecentCommits {
		resp.RecentCommits[i] = CommitInfoResponse{
			Hash:      c.Hash,
			Message:   c.Message,
			Author:    c.Author,
			Timestamp: c.Timestamp,
		}
	}
	return resp
}

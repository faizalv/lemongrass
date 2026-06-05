package transporter

import (
	"github.com/faizalv/lemongrass/modules/fs/entity"
)

type AddProjectRequest struct {
	Path string `json:"path"`
}

func (r AddProjectRequest) ToPayload() string {
	return r.Path
}

type NodeResponse struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Children []NodeResponse `json:"children"`
}

func NodeToResponse(n entity.Node) NodeResponse {
	children := make([]NodeResponse, len(n.Children))
	for i, c := range n.Children {
		children[i] = NodeToResponse(c)
	}
	return NodeResponse{Name: n.Name, Path: n.Path, Children: children}
}

type ProjectResponse struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Branch    string `json:"branch"`
	CreatedAt string `json:"created_at"`
}

type ArtifactResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

func ArtifactToResponse(a entity.Artifact) ArtifactResponse {
	return ArtifactResponse{
		ID:        a.ID,
		Type:      a.Type,
		Name:      a.Name,
		Content:   a.Content,
		Version:   a.Version,
		CreatedAt: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

type ValidateDirResponse struct {
	Ok       bool     `json:"ok"`
	Warnings []string `json:"warnings"`
}

type CreateArtifactRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func ProjectToResponse(p entity.Project) ProjectResponse {
	return ProjectResponse{
		ID:        p.ID,
		Path:      p.Path,
		Status:    p.Status,
		Branch:    p.Branch,
		CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

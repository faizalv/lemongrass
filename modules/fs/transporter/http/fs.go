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
	CreatedAt string `json:"created_at"`
}

func ProjectToResponse(p entity.Project) ProjectResponse {
	return ProjectResponse{
		ID:        p.ID,
		Path:      p.Path,
		Status:    p.Status,
		CreatedAt: p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

package transporter

import (
	"encoding/json"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type CreateJSONRequest struct {
	ProjectID   int64  `json:"project_id"`
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
}

type WorkspaceResponse struct {
	ID              string `json:"id"`
	ProjectID       int64  `json:"project_id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	RequirementType string `json:"requirement_type,omitempty"`
	RequirementText string `json:"requirement_text,omitempty"`
	RequirementFile string `json:"requirement_file,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func ToResponse(ws entity.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:              ws.ID,
		ProjectID:       ws.ProjectID,
		Name:            ws.Name,
		Status:          ws.Status,
		RequirementType: ws.RequirementType,
		RequirementText: ws.RequirementText,
		RequirementFile: ws.RequirementFile,
		CreatedAt:       ws.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       ws.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

type TaskResponse struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	Title       string          `json:"title"`
	Impl        json.RawMessage `json:"impl"`
	Status      string          `json:"status"`
	CreatedAt   string          `json:"created_at"`
	ApprovedAt  *string         `json:"approved_at,omitempty"`
}

type TaskDecisionRequest struct {
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
}

func ToTaskResponse(t entity.Task) TaskResponse {
	r := TaskResponse{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Title:       t.Title,
		Impl:        t.Impl,
		Status:      t.Status,
		CreatedAt:   t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if t.ApprovedAt != nil {
		s := t.ApprovedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.ApprovedAt = &s
	}
	return r
}

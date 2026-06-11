package transporter

import (
	"encoding/json"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type CreateJSONRequest struct {
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
}

type WorkspaceResponse struct {
	ID        string `json:"id"`
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ToResponse(ws entity.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:        ws.ID,
		ProjectID: ws.ProjectID,
		Name:      ws.Name,
		Status:    ws.Status,
		CreatedAt: ws.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: ws.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

type WorkspaceRequirementResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Type        string `json:"type"`
	TextContent string `json:"text_content,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func ToRequirementResponse(r entity.WorkspaceRequirement) WorkspaceRequirementResponse {
	return WorkspaceRequirementResponse{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Type:        r.Type,
		TextContent: r.TextContent,
		FileName:    r.FileName,
		CreatedAt:   r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

type AddTextRequirementRequest struct {
	TextContent string `json:"text_content"`
}

type TaskResponse struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	Title           string          `json:"title"`
	Reason          string          `json:"reason"`
	Impl            json.RawMessage `json:"impl"`
	Status          string          `json:"status"`
	ExecutionStatus string          `json:"execution_status"`
	ExecutionNotes  string          `json:"execution_notes"`
	ExecutionDiff   json.RawMessage `json:"execution_diff,omitempty"`
	RejectionReason string          `json:"rejection_reason"`
	StartedAt       *string         `json:"started_at,omitempty"`
	FinishedAt      *string         `json:"finished_at,omitempty"`
	CreatedAt       string          `json:"created_at"`
	ApprovedAt      *string         `json:"approved_at,omitempty"`
}

type WorkspaceWithRequirementsResponse struct {
	WorkspaceResponse
	Requirements []WorkspaceRequirementResponse `json:"requirements"`
}

type EchoMessageResponse struct {
	Ts   string `json:"ts"`
	Text string `json:"text"`
}

type SessionActivityResponse struct {
	LastActivityAt   *string               `json:"last_activity_at"`
	IdleSeconds      int                   `json:"idle_seconds"`
	Messages         []EchoMessageResponse `json:"messages"`
	CurrentTaskID    *string               `json:"current_task_id,omitempty"`
	CurrentTaskTitle *string               `json:"current_task_title,omitempty"`
}

type TaskDecisionRequest struct {
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
}

func ToTaskResponse(t entity.Task) TaskResponse {
	r := TaskResponse{
		ID:              t.ID,
		WorkspaceID:     t.WorkspaceID,
		Title:           t.Title,
		Reason:          t.Reason,
		Impl:            t.Impl,
		Status:          t.Status,
		ExecutionStatus: t.ExecutionStatus,
		ExecutionNotes:  t.ExecutionNotes,
		ExecutionDiff:   t.ExecutionDiff,
		RejectionReason: t.RejectionReason,
		CreatedAt:       t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if t.StartedAt != nil {
		s := t.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.StartedAt = &s
	}
	if t.FinishedAt != nil {
		s := t.FinishedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.FinishedAt = &s
	}
	if t.ApprovedAt != nil {
		s := t.ApprovedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.ApprovedAt = &s
	}
	return r
}

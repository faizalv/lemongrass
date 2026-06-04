package entity

import (
	"encoding/json"
	"time"
)

type Workspace struct {
	ID        string
	ProjectID int64
	Name      string
	Status    string // idle | grooming | awaiting_execution | executing | done | deleted
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorkspaceRequirement struct {
	ID          string
	WorkspaceID string
	Type        string // text | pdf | image
	TextContent string
	FilePath    string
	FileName    string
	CreatedAt   time.Time
}

type Task struct {
	ID                string
	WorkspaceID       string
	Title             string
	Reason            string
	Impl              json.RawMessage
	Status            string
	AmendmentFeedback string
	CreatedAt         time.Time
	ApprovedAt        *time.Time
}

type TaskDecision struct {
	Approved bool
	Feedback string
}

package entity

import (
	"encoding/json"
	"time"
)

type Workspace struct {
	ID              string
	ProjectID       int64
	Name            string
	Status          string // idle | grooming | awaiting_execution | executing | done
	RequirementText string
	RequirementFile string
	RequirementType string // text | pdf | image
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Task struct {
	ID          string
	WorkspaceID string
	Title       string
	Impl        json.RawMessage
	Status      string
	CreatedAt   time.Time
	ApprovedAt  *time.Time
}

package entity

import "time"

type Artifact struct {
	ID        string
	ProjectID int64
	Type      string
	Name      string
	Content   string
	Version   int
	CreatedAt time.Time
}

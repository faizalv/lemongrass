package entity

import "time"

type Call struct {
	Cmd       string
	Args      string
	Timestamp time.Time
}

type WriteTrailEntry struct {
	SessionID string
	FilePath  string
	ByteCount int
	Timestamp time.Time
}

package entity

import "time"

type Call struct {
	Cmd         string
	Args        string
	Response    string
	SessionID   string
	SessionType string
	DurationMs  int64
	Timestamp   time.Time
}

type EchoMessage struct {
	Timestamp time.Time
	Text      string
}

type WriteTrailEntry struct {
	SessionID string
	FilePath  string
	ByteCount int
	Timestamp time.Time
}

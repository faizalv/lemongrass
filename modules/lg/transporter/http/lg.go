package transporter

import (
	"time"

	"github.com/faizalv/lemongrass/modules/lg/entity"
)

type CallResponse struct {
	Cmd       string    `json:"cmd"`
	Args      string    `json:"args"`
	Timestamp time.Time `json:"timestamp"`
}

func ToCallResponse(c entity.Call) CallResponse {
	return CallResponse{Cmd: c.Cmd, Args: c.Args, Timestamp: c.Timestamp}
}

type WriteTrailResponse struct {
	SessionID string    `json:"session_id"`
	FilePath  string    `json:"file_path"`
	ByteCount int       `json:"byte_count"`
	Timestamp time.Time `json:"timestamp"`
}

type FileDiffResponse struct {
	FilePath     string `json:"file_path"`
	Diff         string `json:"diff"`
	IsNew        bool   `json:"is_new"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
}

func ToFileDiffResponse(d entity.FileDiff) FileDiffResponse {
	return FileDiffResponse{
		FilePath:     d.FilePath,
		Diff:         d.Diff,
		IsNew:        d.IsNew,
		LinesAdded:   d.LinesAdded,
		LinesRemoved: d.LinesRemoved,
	}
}

func ToWriteTrailResponse(e entity.WriteTrailEntry) WriteTrailResponse {
	return WriteTrailResponse{
		SessionID: e.SessionID,
		FilePath:  e.FilePath,
		ByteCount: e.ByteCount,
		Timestamp: e.Timestamp,
	}
}

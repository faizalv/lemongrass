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

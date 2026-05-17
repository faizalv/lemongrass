package entity

import "time"

type Call struct {
	Cmd       string
	Args      string
	Timestamp time.Time
}

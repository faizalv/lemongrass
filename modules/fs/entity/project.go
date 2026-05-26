package entity

import "time"

type Project struct {
	ID        int64
	Path      string
	Status    string
	Branch    string
	CreatedAt time.Time
}

type Node struct {
	Name     string
	Path     string
	Children []Node
}

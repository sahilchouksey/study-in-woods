package model

import "time"

type Status int

const (
	Pending Status = iota
	Done
	UnableToFinish
)

type Todo struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Due         time.Time `json:"due"`
}

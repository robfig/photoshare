package models

import (
	"github.com/coopernurse/gorp"
	"math/rand"
)

type Event struct {
	EventId int32
	Name    string
	Admin   string
}

// Set a random integer key before inserting
func (e *Event) PreInsert(_ gorp.SqlExecutor) error {
	e.EventId = rand.Int31()
	return nil
}

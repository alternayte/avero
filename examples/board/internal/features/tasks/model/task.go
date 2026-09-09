// Package model holds the rows of the tasks feature. drel reads it and writes
// the columns, the repository and the migrations of each type.
package model

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

// Task is one row of the tasks table.
//
// The embedded Model gives the identifier, the times and the change tracking.
// The key is a UUID of version 7, which drel stamps at Add, so the identifier
// stands before the commit and the answer of a handler can carry it.
//
// A db tag names the column. Write a field, then run `avero generate`. Write
// the change of the table with `avero migrate new add_a_column`.
type Task struct {
	drel.Model[uuid.UUID]

	// Title is the text that a person reads.
	Title string `db:"title"`
	// Done states whether the task is complete.
	Done bool `db:"done"`
}

// NewTask returns one open task.
func NewTask(title string) *Task {
	return &Task{Title: title}
}

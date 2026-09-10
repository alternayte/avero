// Package model holds the rows of the posts feature. drel reads it and writes
// the columns, the repository and the migrations of each type.
package model

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

//go:generate go run github.com/alternayte/drel/cmd/drel generate --module posts

// Post is one row of the posts table.
//
// The embedded Model gives the identifier, the times and the change tracking.
// The key is a UUID of version 7, which drel stamps at Add, so the identifier
// stands before the commit and the answer of a handler can carry it.
//
// A db tag names the column. Write a field. Run `avero generate`. Write the
// change of the table with `avero migrate new add_a_column`.
type Post struct {
	drel.Model[uuid.UUID]

	// Title is the name that a person reads.
	Title string `db:"title"`
	// Body is the text of the post.
	Body string `db:"body"`
}

// NewPost returns a post that a person wrote.
func NewPost(title, body string) *Post {
	return &Post{Title: title, Body: body}
}

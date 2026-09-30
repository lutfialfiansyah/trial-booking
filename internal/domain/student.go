package domain

import "time"

// Student represents a child belonging to a parent.
type Student struct {
	ID        int64
	ParentID  int64
	Name      string
	CreatedAt time.Time
}

package domain

import "time"

// Parent represents a paying customer who owns one or more students.
type Parent struct {
	ID        int64
	Name      string
	Email     string
	CreatedAt time.Time
}

// ParentWithChildren is a parent together with their students, used by the
// frontend to let a parent pick one of their children.
type ParentWithChildren struct {
	Parent
	Children []Student
}

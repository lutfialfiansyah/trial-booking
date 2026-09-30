package domain

import "time"

// TrialClass represents a scheduled trial class a student can book.
type TrialClass struct {
	ID        int64
	Title     string
	Subject   string
	StartsAt  time.Time
	Capacity  int
	CreatedAt time.Time
}

// ClassWithAvailability decorates a TrialClass with live availability data.
type ClassWithAvailability struct {
	TrialClass
	ConfirmedCount int
	RemainingSeats int
}

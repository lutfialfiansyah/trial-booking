package domain

import "time"

// RosterEntry is a booking for a class enriched with student and parent
// details, used to render the admin roster. Entries may be in any status.
type RosterEntry struct {
	BookingID   int64
	StudentID   int64
	StudentName string
	ParentName  string
	ParentEmail string
	Status      BookingStatus
	CreatedAt   time.Time
}

// Roster is a trial class together with all of its bookings. ConfirmedCount and
// RemainingSeats reflect only confirmed bookings, since only those consume
// capacity; Entries contains every booking for operational visibility.
type Roster struct {
	Class          TrialClass
	ConfirmedCount int
	RemainingSeats int
	TotalBookings  int
	Entries        []RosterEntry
}

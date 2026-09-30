package http

import (
	"time"

	"trial-booking/internal/domain"
)

// ClassDTO is the transport representation of a trial class.
type ClassDTO struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Subject        string    `json:"subject"`
	StartsAt       time.Time `json:"starts_at"`
	Capacity       int       `json:"capacity"`
	ConfirmedCount int       `json:"confirmed_count"`
	RemainingSeats int       `json:"remaining_seats"`
}

// BookingDTO is the transport representation of a booking.
type BookingDTO struct {
	ID           int64                `json:"id"`
	StudentID    int64                `json:"student_id"`
	TrialClassID int64                `json:"trial_class_id"`
	Status       domain.BookingStatus `json:"status"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// StudentDTO is the transport representation of a student.
type StudentDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ParentWithChildrenDTO is the transport representation of a parent and children.
type ParentWithChildrenDTO struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Email    string       `json:"email"`
	Children []StudentDTO `json:"children"`
}

// RosterEntryDTO is the transport representation of a single roster entry.
type RosterEntryDTO struct {
	BookingID   int64                `json:"booking_id"`
	StudentID   int64                `json:"student_id"`
	StudentName string               `json:"student_name"`
	ParentName  string               `json:"parent_name"`
	ParentEmail string               `json:"parent_email"`
	Status      domain.BookingStatus `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
}

// RosterDTO is the transport representation of a class roster.
type RosterDTO struct {
	Class          ClassDTO         `json:"class"`
	ConfirmedCount int              `json:"confirmed_count"`
	RemainingSeats int              `json:"remaining_seats"`
	TotalBookings  int              `json:"total_bookings"`
	Entries        []RosterEntryDTO `json:"entries"`
}

// CreateBookingRequest is the request payload for creating a booking.
type CreateBookingRequest struct {
	ParentID     int64 `json:"parent_id"`
	StudentID    int64 `json:"student_id"`
	TrialClassID int64 `json:"trial_class_id"`
}

// Validate returns a field->message map; an empty map means valid.
func (r CreateBookingRequest) Validate() map[string]string {
	fields := map[string]string{}
	if r.ParentID <= 0 {
		fields["parent_id"] = "must be greater than 0"
	}
	if r.StudentID <= 0 {
		fields["student_id"] = "must be greater than 0"
	}
	if r.TrialClassID <= 0 {
		fields["trial_class_id"] = "must be greater than 0"
	}
	return fields
}

// PaymentRequest is the request payload for processing a payment.
type PaymentRequest struct {
	Outcome     string `json:"outcome"`
	ProviderRef string `json:"provider_ref"`
}

// Validate returns a field->message map; an empty map means valid.
func (r PaymentRequest) Validate() map[string]string {
	fields := map[string]string{}
	if r.Outcome != "success" && r.Outcome != "failure" {
		fields["outcome"] = "must be either \"success\" or \"failure\""
	}
	return fields
}

// classToDTO maps a ClassWithAvailability to a ClassDTO.
func classToDTO(c domain.ClassWithAvailability) ClassDTO {
	return ClassDTO{
		ID:             c.ID,
		Title:          c.Title,
		Subject:        c.Subject,
		StartsAt:       c.StartsAt,
		Capacity:       c.Capacity,
		ConfirmedCount: c.ConfirmedCount,
		RemainingSeats: c.RemainingSeats,
	}
}

// trialClassToDTO maps a TrialClass to a ClassDTO (counts left at zero).
func trialClassToDTO(c domain.TrialClass) ClassDTO {
	return ClassDTO{
		ID:       c.ID,
		Title:    c.Title,
		Subject:  c.Subject,
		StartsAt: c.StartsAt,
		Capacity: c.Capacity,
	}
}

// bookingToDTO maps a Booking to a BookingDTO.
func bookingToDTO(b domain.Booking) BookingDTO {
	return BookingDTO{
		ID:           b.ID,
		StudentID:    b.StudentID,
		TrialClassID: b.TrialClassID,
		Status:       b.Status,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}

// parentWithChildrenToDTO maps a ParentWithChildren to its DTO.
func parentWithChildrenToDTO(p domain.ParentWithChildren) ParentWithChildrenDTO {
	children := make([]StudentDTO, 0, len(p.Children))
	for _, s := range p.Children {
		children = append(children, StudentDTO{ID: s.ID, Name: s.Name})
	}
	return ParentWithChildrenDTO{
		ID:       p.ID,
		Name:     p.Name,
		Email:    p.Email,
		Children: children,
	}
}

// rosterToDTO maps a Roster to its DTO.
func rosterToDTO(r domain.Roster) RosterDTO {
	entries := make([]RosterEntryDTO, 0, len(r.Entries))
	for _, e := range r.Entries {
		entries = append(entries, RosterEntryDTO{
			BookingID:   e.BookingID,
			StudentID:   e.StudentID,
			StudentName: e.StudentName,
			ParentName:  e.ParentName,
			ParentEmail: e.ParentEmail,
			Status:      e.Status,
			CreatedAt:   e.CreatedAt,
		})
	}
	return RosterDTO{
		Class:          rosterClassDTO(r),
		ConfirmedCount: r.ConfirmedCount,
		RemainingSeats: r.RemainingSeats,
		TotalBookings:  r.TotalBookings,
		Entries:        entries,
	}
}

// rosterClassDTO maps a roster's class while filling in the availability counts
// that were computed for the roster as a whole.
func rosterClassDTO(r domain.Roster) ClassDTO {
	c := trialClassToDTO(r.Class)
	c.ConfirmedCount = r.ConfirmedCount
	c.RemainingSeats = r.RemainingSeats
	return c
}

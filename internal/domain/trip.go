package domain

import "time"

type TripStatus string

const (
	TripStatusPending    TripStatus = "pending"     // created by the business, no driver yet
	TripStatusAssigned   TripStatus = "assigned"    // driver assigned, not started
	TripStatusInProgress TripStatus = "in_progress" // en route, accepts telemetry
	TripStatusCompleted  TripStatus = "completed"
	TripStatusCancelled  TripStatus = "cancelled"
)

type Trip struct {
	ID        string         `json:"id"`
	DriverID  string         `json:"driver_id,omitempty"`
	Metadata  map[string]any `json:"metadata"`
	Status    TripStatus     `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
}

// CreateTripRequest is created by the business app (e.g. a transfer between branches).
// The driver is assigned later with UpdateTripRequest.
type CreateTripRequest struct {
	Metadata map[string]any `json:"metadata"`
}

type UpdateTripRequest struct {
	DriverID *string        `json:"driver_id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Status   *TripStatus    `json:"status,omitempty"`
}

func (s TripStatus) IsValid() bool {
	switch s {
	case TripStatusPending, TripStatusAssigned, TripStatusInProgress, TripStatusCompleted, TripStatusCancelled:
		return true
	default:
		return false
	}
}

func (s TripStatus) AcceptsTelemetry() bool {
	return s == TripStatusInProgress
}

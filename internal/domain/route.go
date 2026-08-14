package domain

import "time"

type Route struct {
	DriverID   string         `json:"driver_id,omitempty"`
	TripID     string         `json:"trip_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	PointCount int            `json:"point_count"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	EndedAt    *time.Time     `json:"ended_at,omitempty"`
	Points     []Location     `json:"points"`
}

package domain

import "time"

type Location struct {
	ID        int64     `json:"id,omitempty"`
	DriverID  string    `json:"driver_id"`
	TripID    string    `json:"trip_id,omitempty"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Speed     float64   `json:"speed"`
	Heading   float64   `json:"heading"`
	Timestamp time.Time `json:"timestamp"`
}
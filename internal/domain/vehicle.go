package domain

import "time"

type Vehicle struct {
	ID           string    `json:"id"`
	Plate        string    `json:"plate"`
	Model        string    `json:"model"`
	DriverID     string    `json:"driver_id"`
	IsActive     bool      `json:"is_active"`
	LastUpdateAt time.Time `json:"last_update_at"`
}
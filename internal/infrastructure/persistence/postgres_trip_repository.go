package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tracko/internal/domain"
	"tracko/internal/ports"
)

type PostgresTripRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTripRepository(pool *pgxpool.Pool) ports.TripRepository {
	return &PostgresTripRepository{pool: pool}
}

func (r *PostgresTripRepository) Create(ctx context.Context, trip *domain.Trip) error {
	metadata, err := json.Marshal(trip.Metadata)
	if err != nil {
		return fmt.Errorf("error marshaling trip metadata: %w", err)
	}

	query := `
		INSERT INTO trips (id, driver_id, metadata, status, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = r.pool.Exec(ctx, query,
		trip.ID,
		nullIfEmpty(trip.DriverID),
		metadata,
		trip.Status,
		trip.StartedAt,
		trip.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("error creating trip: %w", err)
	}

	return nil
}

func (r *PostgresTripRepository) GetByID(ctx context.Context, tripID string) (*domain.Trip, error) {
	query := `
		SELECT id, driver_id, metadata, status, started_at, ended_at
		FROM trips
		WHERE id = $1
	`

	var trip domain.Trip
	var metadataRaw []byte
	var driverID *string
	err := r.pool.QueryRow(ctx, query, tripID).Scan(
		&trip.ID,
		&driverID,
		&metadataRaw,
		&trip.Status,
		&trip.StartedAt,
		&trip.EndedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting trip %s: %w", tripID, err)
	}
	if driverID != nil {
		trip.DriverID = *driverID
	}

	if err := json.Unmarshal(metadataRaw, &trip.Metadata); err != nil {
		return nil, fmt.Errorf("error unmarshaling trip metadata: %w", err)
	}
	if trip.Metadata == nil {
		trip.Metadata = map[string]any{}
	}

	return &trip, nil
}

func (r *PostgresTripRepository) ListByDriver(ctx context.Context, driverID string) ([]domain.Trip, error) {
	query := `
		SELECT id, driver_id, metadata, status, started_at, ended_at
		FROM trips
		WHERE driver_id = $1
		ORDER BY started_at DESC
	`

	rows, err := r.pool.Query(ctx, query, driverID)
	if err != nil {
		return nil, fmt.Errorf("error listing trips for driver %s: %w", driverID, err)
	}
	defer rows.Close()

	trips := make([]domain.Trip, 0)
	for rows.Next() {
		var trip domain.Trip
		var metadataRaw []byte
		var driverID *string
		if err := rows.Scan(
			&trip.ID,
			&driverID,
			&metadataRaw,
			&trip.Status,
			&trip.StartedAt,
			&trip.EndedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning trip: %w", err)
		}
		if driverID != nil {
			trip.DriverID = *driverID
		}
		if err := json.Unmarshal(metadataRaw, &trip.Metadata); err != nil {
			return nil, fmt.Errorf("error unmarshaling trip metadata: %w", err)
		}
		if trip.Metadata == nil {
			trip.Metadata = map[string]any{}
		}
		trips = append(trips, trip)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trips: %w", err)
	}

	return trips, nil
}

func (r *PostgresTripRepository) Update(ctx context.Context, tripID string, req domain.UpdateTripRequest) (*domain.Trip, error) {
	current, err := r.GetByID(ctx, tripID)
	if err != nil {
		return nil, err
	}

	metadata := current.Metadata
	if req.Metadata != nil {
		for key, value := range req.Metadata {
			metadata[key] = value
		}
	}

	status := current.Status
	if req.Status != nil {
		status = *req.Status
	}

	driverID := current.DriverID
	if req.DriverID != nil {
		driverID = *req.DriverID
	}
	if status == domain.TripStatusAssigned && driverID != "" && current.Status == domain.TripStatusPending {
		status = domain.TripStatusAssigned
	}

	var endedAt *time.Time
	if (status == domain.TripStatusCompleted || status == domain.TripStatusCancelled) && current.EndedAt == nil {
		now := time.Now().UTC()
		endedAt = &now
	} else {
		endedAt = current.EndedAt
	}

	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("error marshaling trip metadata: %w", err)
	}

	query := `
		UPDATE trips
		SET driver_id = NULLIF($2, ''), metadata = $3, status = $4, ended_at = $5
		WHERE id = $1
		RETURNING id, driver_id, metadata, status, started_at, ended_at
	`

	var trip domain.Trip
	var updatedMetadataRaw []byte
	var updatedDriverID *string
	err = r.pool.QueryRow(ctx, query, tripID, driverID, metadataRaw, status, endedAt).Scan(
		&trip.ID,
		&updatedDriverID,
		&updatedMetadataRaw,
		&trip.Status,
		&trip.StartedAt,
		&trip.EndedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("trip %s not found", tripID)
		}
		return nil, fmt.Errorf("error updating trip %s: %w", tripID, err)
	}
	if updatedDriverID != nil {
		trip.DriverID = *updatedDriverID
	}

	if err := json.Unmarshal(updatedMetadataRaw, &trip.Metadata); err != nil {
		return nil, fmt.Errorf("error unmarshaling trip metadata: %w", err)
	}
	if trip.Metadata == nil {
		trip.Metadata = map[string]any{}
	}

	return &trip, nil
}

func nullIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tracko/internal/domain"
	"tracko/internal/ports"
)

type PostgresLocationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLocationRepository(pool *pgxpool.Pool) ports.LocationRepository {
	return &PostgresLocationRepository{pool: pool}
}

func (r *PostgresLocationRepository) Save(ctx context.Context, loc *domain.Location) error {
	query := `
		INSERT INTO vehicle_locations (driver_id, trip_id, coordinates, speed, heading, created_at)
		VALUES ($1, NULLIF($2, ''), ST_SetSRID(ST_MakePoint($3, $4), 4326), $5, $6, $7)
		RETURNING id
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		loc.DriverID,
		loc.TripID,
		loc.Longitude,
		loc.Latitude,
		loc.Speed,
		loc.Heading,
		loc.Timestamp,
	).Scan(&loc.ID)

	if err != nil {
		return fmt.Errorf("error saving location to postgres: %w", err)
	}

	return nil
}

func (r *PostgresLocationRepository) GetLatestByDriver(ctx context.Context, driverID string) (*domain.Location, error) {
	query := `
		SELECT 
			id, 
			driver_id,
			COALESCE(trip_id, '') AS trip_id,
			ST_Y(coordinates::geometry) AS latitude, 
			ST_X(coordinates::geometry) AS longitude, 
			speed, 
			heading, 
			created_at
		FROM vehicle_locations
		WHERE driver_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var loc domain.Location
	err := r.pool.QueryRow(ctx, query, driverID).Scan(
		&loc.ID,
		&loc.DriverID,
		&loc.TripID,
		&loc.Latitude,
		&loc.Longitude,
		&loc.Speed,
		&loc.Heading,
		&loc.Timestamp,
	)

	if err != nil {
		return nil, fmt.Errorf("error getting latest location for driver_id %s: %w", driverID, err)
	}

	return &loc, nil
}

func (r *PostgresLocationRepository) GetRouteByDriver(ctx context.Context, driverID string, from, to *time.Time) ([]domain.Location, error) {
	query := `
		SELECT
			id,
			driver_id,
			COALESCE(trip_id, '') AS trip_id,
			ST_Y(coordinates::geometry) AS latitude,
			ST_X(coordinates::geometry) AS longitude,
			speed,
			heading,
			created_at
		FROM vehicle_locations
		WHERE driver_id = $1
		  AND ($2::timestamptz IS NULL OR created_at >= $2)
		  AND ($3::timestamptz IS NULL OR created_at <= $3)
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, driverID, from, to)
	if err != nil {
		return nil, fmt.Errorf("error querying route for driver_id %s: %w", driverID, err)
	}
	defer rows.Close()

	points := make([]domain.Location, 0)
	for rows.Next() {
		var loc domain.Location
		if err := rows.Scan(
			&loc.ID,
			&loc.DriverID,
			&loc.TripID,
			&loc.Latitude,
			&loc.Longitude,
			&loc.Speed,
			&loc.Heading,
			&loc.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("error scanning route point for driver_id %s: %w", driverID, err)
		}
		points = append(points, loc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating route for driver_id %s: %w", driverID, err)
	}

	return points, nil
}

func (r *PostgresLocationRepository) GetRouteByTrip(ctx context.Context, tripID string) ([]domain.Location, error) {
	query := `
		SELECT
			id,
			driver_id,
			COALESCE(trip_id, '') AS trip_id,
			ST_Y(coordinates::geometry) AS latitude,
			ST_X(coordinates::geometry) AS longitude,
			speed,
			heading,
			created_at
		FROM vehicle_locations
		WHERE trip_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("error querying route for trip_id %s: %w", tripID, err)
	}
	defer rows.Close()

	points := make([]domain.Location, 0)
	for rows.Next() {
		var loc domain.Location
		if err := rows.Scan(
			&loc.ID,
			&loc.DriverID,
			&loc.TripID,
			&loc.Latitude,
			&loc.Longitude,
			&loc.Speed,
			&loc.Heading,
			&loc.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("error scanning route point for trip_id %s: %w", tripID, err)
		}
		points = append(points, loc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating route for trip_id %s: %w", tripID, err)
	}

	return points, nil
}
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS trips (
    id TEXT PRIMARY KEY,
    driver_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS vehicle_locations (
    id BIGSERIAL PRIMARY KEY,
    driver_id TEXT NOT NULL,
    trip_id TEXT REFERENCES trips (id),
    coordinates geometry(Point, 4326) NOT NULL,
    speed DOUBLE PRECISION,
    heading DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trips_driver_started_at
    ON trips (driver_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_vehicle_locations_driver_created_at
    ON vehicle_locations (driver_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_vehicle_locations_trip_created_at
    ON vehicle_locations (trip_id, created_at ASC);

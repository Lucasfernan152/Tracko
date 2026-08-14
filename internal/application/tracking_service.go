package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"tracko/internal/domain"
	"tracko/internal/ports"
)

type TrackingService interface {
	ProcessTelemetry(ctx context.Context, loc *domain.Location) error
	GetLastKnownLocation(ctx context.Context, driverID string) (*domain.Location, error)
	GetRoute(ctx context.Context, driverID string, from, to *time.Time) (*domain.Route, error)
	CreateTrip(ctx context.Context, req domain.CreateTripRequest) (*domain.Trip, error)
	GetTrip(ctx context.Context, tripID string) (*domain.Trip, error)
	ListTripsByDriver(ctx context.Context, driverID string) ([]domain.Trip, error)
	UpdateTrip(ctx context.Context, tripID string, req domain.UpdateTripRequest) (*domain.Trip, error)
	GetTripRoute(ctx context.Context, tripID string) (*domain.Route, error)
}

type trackingService struct {
	repo        ports.LocationRepository
	tripRepo    ports.TripRepository
	broadcaster ports.EventBroadcaster
}

func NewTrackingService(repo ports.LocationRepository, tripRepo ports.TripRepository, broadcaster ports.EventBroadcaster) TrackingService {
	return &trackingService{
		repo:        repo,
		tripRepo:    tripRepo,
		broadcaster: broadcaster,
	}
}

func (s *trackingService) ProcessTelemetry(ctx context.Context, loc *domain.Location) error {
	if loc.TripID == "" {
		return fmt.Errorf("trip_id is required")
	}
	if loc.Latitude < -90 || loc.Latitude > 90 || loc.Longitude < -180 || loc.Longitude > 180 {
		return fmt.Errorf("invalid geographic coordinates: lat %f, lng %f", loc.Latitude, loc.Longitude)
	}

	trip, err := s.tripRepo.GetByID(ctx, loc.TripID)
	if err != nil {
		return fmt.Errorf("invalid trip_id: %w", err)
	}
	if !trip.Status.AcceptsTelemetry() {
		return fmt.Errorf("trip %s is not in progress (status: %s)", loc.TripID, trip.Status)
	}
	if trip.DriverID == "" {
		return fmt.Errorf("trip %s has no assigned driver", loc.TripID)
	}
	if loc.DriverID != "" && loc.DriverID != trip.DriverID {
		return fmt.Errorf("driver %s is not assigned to trip %s", loc.DriverID, loc.TripID)
	}

	loc.DriverID = trip.DriverID

	if err := s.repo.Save(ctx, loc); err != nil {
		return fmt.Errorf("error processing telemetry: %w", err)
	}

	log.Printf("[TrackingService] Location saved for trip %s driver %s (Lat: %f, Lng: %f)",
		loc.TripID, loc.DriverID, loc.Latitude, loc.Longitude)

	if s.broadcaster != nil {
		s.broadcaster.BroadcastLocation(loc)
	}

	return nil
}

func (s *trackingService) GetLastKnownLocation(ctx context.Context, driverID string) (*domain.Location, error) {
	if driverID == "" {
		return nil, fmt.Errorf("driver_id cannot be empty")
	}

	return s.repo.GetLatestByDriver(ctx, driverID)
}

func (s *trackingService) GetRoute(ctx context.Context, driverID string, from, to *time.Time) (*domain.Route, error) {
	if driverID == "" {
		return nil, fmt.Errorf("driver_id cannot be empty")
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, fmt.Errorf("from cannot be after to")
	}

	points, err := s.repo.GetRouteByDriver(ctx, driverID, from, to)
	if err != nil {
		return nil, fmt.Errorf("error getting route for driver_id %s: %w", driverID, err)
	}

	return buildRoute(driverID, "", nil, points), nil
}

func (s *trackingService) CreateTrip(ctx context.Context, req domain.CreateTripRequest) (*domain.Trip, error) {
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	trip := &domain.Trip{
		ID:        newTripID(),
		Metadata:  metadata,
		Status:    domain.TripStatusPending,
		StartedAt: time.Now().UTC(),
	}

	if err := s.tripRepo.Create(ctx, trip); err != nil {
		return nil, fmt.Errorf("error creating trip: %w", err)
	}

	return trip, nil
}

func (s *trackingService) GetTrip(ctx context.Context, tripID string) (*domain.Trip, error) {
	if tripID == "" {
		return nil, fmt.Errorf("trip_id cannot be empty")
	}
	return s.tripRepo.GetByID(ctx, tripID)
}

func (s *trackingService) ListTripsByDriver(ctx context.Context, driverID string) ([]domain.Trip, error) {
	if driverID == "" {
		return nil, fmt.Errorf("driver_id cannot be empty")
	}
	return s.tripRepo.ListByDriver(ctx, driverID)
}

func (s *trackingService) UpdateTrip(ctx context.Context, tripID string, req domain.UpdateTripRequest) (*domain.Trip, error) {
	if tripID == "" {
		return nil, fmt.Errorf("trip_id cannot be empty")
	}
	if req.Status != nil && !req.Status.IsValid() {
		return nil, fmt.Errorf("invalid trip status")
	}

	current, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return nil, err
	}

	if req.DriverID != nil && *req.DriverID != "" {
		if current.Status == domain.TripStatusCompleted || current.Status == domain.TripStatusCancelled {
			return nil, fmt.Errorf("cannot assign driver to a %s trip", current.Status)
		}
		if current.Status == domain.TripStatusPending && req.Status == nil {
			assigned := domain.TripStatusAssigned
			req.Status = &assigned
		}
	}

	if req.Status != nil {
		if err := validateStatusTransition(current, req); err != nil {
			return nil, err
		}
	}

	return s.tripRepo.Update(ctx, tripID, req)
}

func validateStatusTransition(current *domain.Trip, req domain.UpdateTripRequest) error {
	next := *req.Status
	driverID := current.DriverID
	if req.DriverID != nil && *req.DriverID != "" {
		driverID = *req.DriverID
	}

	switch next {
	case domain.TripStatusAssigned:
		if driverID == "" {
			return fmt.Errorf("driver_id is required to assign a trip")
		}
	case domain.TripStatusInProgress:
		if driverID == "" {
			return fmt.Errorf("trip must have an assigned driver before starting")
		}
		if current.Status != domain.TripStatusAssigned && current.Status != domain.TripStatusPending {
			return fmt.Errorf("trip cannot start from status %s", current.Status)
		}
	case domain.TripStatusCompleted:
		if current.Status != domain.TripStatusInProgress && current.Status != domain.TripStatusAssigned {
			return fmt.Errorf("trip cannot be completed from status %s", current.Status)
		}
	case domain.TripStatusCancelled:
		if current.Status == domain.TripStatusCompleted {
			return fmt.Errorf("completed trips cannot be cancelled")
		}
	}

	return nil
}

func (s *trackingService) GetTripRoute(ctx context.Context, tripID string) (*domain.Route, error) {
	if tripID == "" {
		return nil, fmt.Errorf("trip_id cannot be empty")
	}

	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("error getting trip %s: %w", tripID, err)
	}

	points, err := s.repo.GetRouteByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("error getting route for trip_id %s: %w", tripID, err)
	}

	return buildRoute(trip.DriverID, trip.ID, trip.Metadata, points), nil
}

func buildRoute(driverID, tripID string, metadata map[string]any, points []domain.Location) *domain.Route {
	route := &domain.Route{
		DriverID:   driverID,
		TripID:     tripID,
		Metadata:   metadata,
		PointCount: len(points),
		Points:     points,
	}
	if len(points) > 0 {
		start := points[0].Timestamp
		end := points[len(points)-1].Timestamp
		route.StartedAt = &start
		route.EndedAt = &end
	}
	return route
}

func newTripID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "trip-" + hex.EncodeToString(b[:])
}

package ports

import (
	"context"

	"tracko/internal/domain"
)

type TripRepository interface {
	Create(ctx context.Context, trip *domain.Trip) error
	GetByID(ctx context.Context, tripID string) (*domain.Trip, error)
	ListByDriver(ctx context.Context, driverID string) ([]domain.Trip, error)
	Update(ctx context.Context, tripID string, req domain.UpdateTripRequest) (*domain.Trip, error)
}

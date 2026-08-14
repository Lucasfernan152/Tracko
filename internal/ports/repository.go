package ports

import (
	"context"
	"time"

	"tracko/internal/domain"
)

type LocationRepository interface {
	Save(ctx context.Context, loc *domain.Location) error
	GetLatestByDriver(ctx context.Context, driverID string) (*domain.Location, error)
	GetRouteByDriver(ctx context.Context, driverID string, from, to *time.Time) ([]domain.Location, error)
	GetRouteByTrip(ctx context.Context, tripID string) ([]domain.Location, error)
}
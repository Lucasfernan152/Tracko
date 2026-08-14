package ports

import "context"

type TelemetrySubscriber interface {
	StartConsuming(ctx context.Context) error
	Close()
}
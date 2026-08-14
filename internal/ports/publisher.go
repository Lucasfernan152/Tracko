package ports

import (

	"tracko/internal/domain"
)

type EventBroadcaster interface {
	BroadcastLocation(loc *domain.Location)
}
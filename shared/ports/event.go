package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

type EventProducer interface {
	SendBrandSearch(ctx context.Context, event domain.BrandMoniterEvent) error
}

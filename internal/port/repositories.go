package port

import (
	"context"
	"time"

	"github.com/freeluncher/rentalin-backend/internal/domain"
)

type InventoryRepository interface {
	ReserveAvailableItems(ctx context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error)
	TransitionItemsStatus(ctx context.Context, tenantID string, itemIDs []string, from, to domain.ItemAvailabilityStatus) error
	SetItemStatus(ctx context.Context, tenantID, itemID string, to domain.ItemAvailabilityStatus) error
	GetByIDs(ctx context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error)
}

type RentalRepository interface {
	Create(ctx context.Context, rental domain.Rental) (domain.Rental, error)
	GetByID(ctx context.Context, tenantID, rentalID string) (domain.Rental, error)
	ListByTenant(ctx context.Context, tenantID string) ([]domain.Rental, error)
	Update(ctx context.Context, rental domain.Rental) (domain.Rental, error)
	HasActiveScheduleConflict(ctx context.Context, tenantID string, itemIDs []string, fromAt, toAt time.Time, excludeRentalID string) (bool, error)
}

type AuditLogEntry struct {
	TenantID   string
	ActorUser  string
	Action     string
	Entity     string
	EntityID   string
	OccurredAt time.Time
	Payload    map[string]any
}

type AuditRepository interface {
	Append(ctx context.Context, entry AuditLogEntry) error
}

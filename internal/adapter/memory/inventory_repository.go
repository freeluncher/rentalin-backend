package memory

import (
	"context"
	"sync"

	"github.com/freeluncher/rentalin-backend/internal/domain"
)

type InventoryRepository struct {
	mu    sync.Mutex
	items map[string]domain.ProductItem
}

func NewInventoryRepository(seed []domain.ProductItem) *InventoryRepository {
	items := make(map[string]domain.ProductItem, len(seed))
	for _, item := range seed {
		items[item.ID] = item
	}
	return &InventoryRepository{items: items}
}

func (r *InventoryRepository) ReserveAvailableItems(_ context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	reserved := make([]domain.ProductItem, 0, len(itemIDs))
	for _, id := range itemIDs {
		item, ok := r.items[id]
		if !ok || item.TenantID != tenantID || item.AvailabilityStatus != domain.ItemStatusAvailable {
			return nil, domain.ErrItemUnavailable
		}
		reserved = append(reserved, item)
	}

	for _, item := range reserved {
		item.AvailabilityStatus = domain.ItemStatusReserved
		r.items[item.ID] = item
	}

	return reserved, nil
}

func (r *InventoryRepository) TransitionItemsStatus(_ context.Context, tenantID string, itemIDs []string, from, to domain.ItemAvailabilityStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range itemIDs {
		item, ok := r.items[id]
		if !ok || item.TenantID != tenantID || item.AvailabilityStatus != from {
			return domain.ErrItemUnavailable
		}
	}

	for _, id := range itemIDs {
		item := r.items[id]
		item.AvailabilityStatus = to
		r.items[id] = item
	}
	return nil
}

func (r *InventoryRepository) SetItemStatus(_ context.Context, tenantID, itemID string, to domain.ItemAvailabilityStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[itemID]
	if !ok || item.TenantID != tenantID {
		return domain.ErrItemUnavailable
	}
	item.AvailabilityStatus = to
	r.items[itemID] = item
	return nil
}

func (r *InventoryRepository) GetByIDs(_ context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]domain.ProductItem, 0, len(itemIDs))
	for _, id := range itemIDs {
		item, ok := r.items[id]
		if !ok || item.TenantID != tenantID {
			return nil, domain.ErrItemUnavailable
		}
		items = append(items, item)
	}
	return items, nil
}

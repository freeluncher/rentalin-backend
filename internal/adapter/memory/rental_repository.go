package memory

import (
	"context"
	"sync"

	"github.com/freeluncher/rentalin-backend/internal/domain"
)

type RentalRepository struct {
	mu      sync.RWMutex
	rentals map[string]domain.Rental
}

func NewRentalRepository() *RentalRepository {
	return &RentalRepository{rentals: map[string]domain.Rental{}}
}

func (r *RentalRepository) Create(_ context.Context, rental domain.Rental) (domain.Rental, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rentals[rental.ID] = rental
	return rental, nil
}

func (r *RentalRepository) GetByID(_ context.Context, tenantID, rentalID string) (domain.Rental, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rental, ok := r.rentals[rentalID]
	if !ok || rental.TenantID != tenantID {
		return domain.Rental{}, domain.ErrRentalNotFound
	}
	return rental, nil
}

func (r *RentalRepository) ListByTenant(_ context.Context, tenantID string) ([]domain.Rental, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Rental, 0)
	for _, rental := range r.rentals {
		if rental.TenantID == tenantID {
			out = append(out, rental)
		}
	}
	return out, nil
}

func (r *RentalRepository) Update(_ context.Context, rental domain.Rental) (domain.Rental, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.rentals[rental.ID]
	if !ok {
		return domain.Rental{}, domain.ErrRentalNotFound
	}
	r.rentals[rental.ID] = rental
	return rental, nil
}

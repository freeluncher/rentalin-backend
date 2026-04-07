package memory

import (
	"context"
	"sync"

	"github.com/freeluncher/rentalin-backend/internal/port"
)

type AuditRepository struct {
	mu      sync.Mutex
	entries []port.AuditLogEntry
}

func NewAuditRepository() *AuditRepository {
	return &AuditRepository{entries: make([]port.AuditLogEntry, 0, 32)}
}

func (r *AuditRepository) Append(_ context.Context, entry port.AuditLogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

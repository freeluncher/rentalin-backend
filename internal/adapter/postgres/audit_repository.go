package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freeluncher/rentalin-backend/internal/port"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Append(ctx context.Context, entry port.AuditLogEntry) error {
	payload, err := json.Marshal(entry.Payload)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (
			tenant_id, actor_user_id, action, entity, entity_id, payload_jsonb, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, entry.TenantID, entry.ActorUser, entry.Action, entry.Entity, entry.EntityID, payload, entry.OccurredAt)
	return err
}

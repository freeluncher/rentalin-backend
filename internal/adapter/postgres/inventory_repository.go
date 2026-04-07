package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freeluncher/rentalin-backend/internal/domain"
)

type InventoryRepository struct {
	pool *pgxpool.Pool
}

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

func (r *InventoryRepository) ReserveAvailableItems(ctx context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	items, err := fetchItemsForUpdate(ctx, tx, tenantID, itemIDs)
	if err != nil {
		return nil, err
	}
	if len(items) != len(itemIDs) {
		return nil, domain.ErrItemUnavailable
	}
	for _, item := range items {
		if item.AvailabilityStatus != domain.ItemStatusAvailable {
			return nil, domain.ErrItemUnavailable
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE product_items
		SET availability_status = $1
		WHERE tenant_id = $2 AND id = ANY($3)
	`, string(domain.ItemStatusReserved), tenantID, itemIDs)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].AvailabilityStatus = domain.ItemStatusReserved
	}
	return items, nil
}

func (r *InventoryRepository) TransitionItemsStatus(ctx context.Context, tenantID string, itemIDs []string, from, to domain.ItemAvailabilityStatus) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	items, err := fetchItemsForUpdate(ctx, tx, tenantID, itemIDs)
	if err != nil {
		return err
	}
	if len(items) != len(itemIDs) {
		return domain.ErrItemUnavailable
	}
	for _, item := range items {
		if item.AvailabilityStatus != from {
			return domain.ErrItemUnavailable
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE product_items
		SET availability_status = $1
		WHERE tenant_id = $2 AND id = ANY($3)
	`, string(to), tenantID, itemIDs)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *InventoryRepository) SetItemStatus(ctx context.Context, tenantID, itemID string, to domain.ItemAvailabilityStatus) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE product_items
		SET availability_status = $1
		WHERE tenant_id = $2 AND id = $3
	`, string(to), tenantID, itemID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrItemUnavailable
	}
	return nil
}

func (r *InventoryRepository) GetByIDs(ctx context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, product_id, serial_number, condition_status, availability_status
		FROM product_items
		WHERE tenant_id = $1 AND id = ANY($2)
	`, tenantID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductItem, 0, len(itemIDs))
	for rows.Next() {
		var item domain.ProductItem
		var status string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProductID, &item.SerialNumber, &item.ConditionStatus, &status); err != nil {
			return nil, err
		}
		item.AvailabilityStatus = domain.ItemAvailabilityStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) != len(itemIDs) {
		return nil, domain.ErrItemUnavailable
	}
	return items, nil
}

func fetchItemsForUpdate(ctx context.Context, tx pgx.Tx, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, product_id, serial_number, condition_status, availability_status
		FROM product_items
		WHERE tenant_id = $1 AND id = ANY($2)
		FOR UPDATE
	`, tenantID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ProductItem, 0, len(itemIDs))
	for rows.Next() {
		var item domain.ProductItem
		var status string
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ProductID, &item.SerialNumber, &item.ConditionStatus, &status); err != nil {
			return nil, err
		}
		item.AvailabilityStatus = domain.ItemAvailabilityStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

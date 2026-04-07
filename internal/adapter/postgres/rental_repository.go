package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freeluncher/rentalin-backend/internal/domain"
)

type RentalRepository struct {
	pool *pgxpool.Pool
}

func NewRentalRepository(pool *pgxpool.Pool) *RentalRepository {
	return &RentalRepository{pool: pool}
}

func (r *RentalRepository) Create(ctx context.Context, rental domain.Rental) (domain.Rental, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Rental{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO rentals (
			id, tenant_id, customer_name, start_at, due_at, returned_at,
			status, subtotal, total_fees, grand_total, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		rental.ID,
		rental.TenantID,
		rental.CustomerName,
		rental.StartAt,
		rental.DueAt,
		rental.ReturnedAt,
		string(rental.Status),
		rental.Subtotal,
		rental.TotalFees,
		rental.GrandTotal,
		rental.CreatedAt,
		rental.UpdatedAt,
	)
	if err != nil {
		return domain.Rental{}, err
	}

	if err := upsertRentalItems(ctx, tx, rental.TenantID, rental.ID, rental.RentalItems); err != nil {
		return domain.Rental{}, err
	}
	if err := upsertFeeLines(ctx, tx, rental.TenantID, rental.ID, rental.FeeLines); err != nil {
		return domain.Rental{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Rental{}, err
	}
	return rental, nil
}

func (r *RentalRepository) GetByID(ctx context.Context, tenantID, rentalID string) (domain.Rental, error) {
	rental, err := fetchRental(ctx, r.pool, tenantID, rentalID)
	if err != nil {
		return domain.Rental{}, err
	}
	return rental, nil
}

func (r *RentalRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.Rental, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM rentals
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rentals := make([]domain.Rental, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		rental, err := fetchRental(ctx, r.pool, tenantID, id)
		if err != nil {
			return nil, err
		}
		rentals = append(rentals, rental)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rentals, nil
}

func (r *RentalRepository) Update(ctx context.Context, rental domain.Rental) (domain.Rental, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Rental{}, err
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `
		UPDATE rentals
		SET customer_name = $1,
			start_at = $2,
			due_at = $3,
			returned_at = $4,
			status = $5,
			subtotal = $6,
			total_fees = $7,
			grand_total = $8,
			updated_at = $9
		WHERE id = $10 AND tenant_id = $11
	`,
		rental.CustomerName,
		rental.StartAt,
		rental.DueAt,
		rental.ReturnedAt,
		string(rental.Status),
		rental.Subtotal,
		rental.TotalFees,
		rental.GrandTotal,
		rental.UpdatedAt,
		rental.ID,
		rental.TenantID,
	)
	if err != nil {
		return domain.Rental{}, err
	}
	if cmd.RowsAffected() == 0 {
		return domain.Rental{}, domain.ErrRentalNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM rental_items WHERE rental_id = $1 AND tenant_id = $2`, rental.ID, rental.TenantID); err != nil {
		return domain.Rental{}, err
	}
	if err := upsertRentalItems(ctx, tx, rental.TenantID, rental.ID, rental.RentalItems); err != nil {
		return domain.Rental{}, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM fee_lines WHERE rental_id = $1 AND tenant_id = $2`, rental.ID, rental.TenantID); err != nil {
		return domain.Rental{}, err
	}
	if err := upsertFeeLines(ctx, tx, rental.TenantID, rental.ID, rental.FeeLines); err != nil {
		return domain.Rental{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Rental{}, err
	}
	return rental, nil
}

func fetchRental(ctx context.Context, q queryer, tenantID, rentalID string) (domain.Rental, error) {
	var rental domain.Rental
	var status string
	err := q.QueryRow(ctx, `
		SELECT id, tenant_id, customer_name, start_at, due_at, returned_at,
		       status, subtotal, total_fees, grand_total, created_at, updated_at
		FROM rentals
		WHERE id = $1 AND tenant_id = $2
	`, rentalID, tenantID).Scan(
		&rental.ID,
		&rental.TenantID,
		&rental.CustomerName,
		&rental.StartAt,
		&rental.DueAt,
		&rental.ReturnedAt,
		&status,
		&rental.Subtotal,
		&rental.TotalFees,
		&rental.GrandTotal,
		&rental.CreatedAt,
		&rental.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Rental{}, domain.ErrRentalNotFound
		}
		return domain.Rental{}, err
	}
	rental.Status = domain.RentalStatus(status)

	items, err := fetchRentalItems(ctx, q, tenantID, rental.ID)
	if err != nil {
		return domain.Rental{}, err
	}
	fees, err := fetchFeeLines(ctx, q, tenantID, rental.ID)
	if err != nil {
		return domain.Rental{}, err
	}

	rental.RentalItems = items
	rental.FeeLines = fees
	return rental, nil
}

func fetchRentalItems(ctx context.Context, q queryer, tenantID, rentalID string) ([]domain.RentalItem, error) {
	rows, err := q.Query(ctx, `
		SELECT id, product_item_id, daily_rate, planned_days, actual_days, line_total,
		       status, returned_at, return_condition, damage_assessment
		FROM rental_items
		WHERE tenant_id = $1 AND rental_id = $2
		ORDER BY created_at ASC
	`, tenantID, rentalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RentalItem, 0)
	for rows.Next() {
		var item domain.RentalItem
		var status string
		if err := rows.Scan(
			&item.ID,
			&item.ProductItemID,
			&item.DailyRate,
			&item.PlannedDays,
			&item.ActualDays,
			&item.LineTotal,
			&status,
			&item.ReturnedAt,
			&item.ReturnCondition,
			&item.DamageAssessment,
		); err != nil {
			return nil, err
		}
		item.Status = domain.RentalItemStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func fetchFeeLines(ctx context.Context, q queryer, tenantID, rentalID string) ([]domain.FeeLine, error) {
	rows, err := q.Query(ctx, `
		SELECT id, rental_id, rental_item_id, fee_type, amount, notes, created_by, created_at
		FROM fee_lines
		WHERE tenant_id = $1 AND rental_id = $2
		ORDER BY created_at ASC
	`, tenantID, rentalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fees := make([]domain.FeeLine, 0)
	for rows.Next() {
		var fee domain.FeeLine
		var feeType string
		if err := rows.Scan(&fee.ID, &fee.RentalID, &fee.RentalItemID, &feeType, &fee.Amount, &fee.Notes, &fee.CreatedBy, &fee.CreatedAt); err != nil {
			return nil, err
		}
		fee.FeeType = domain.FeeType(feeType)
		fees = append(fees, fee)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fees, nil
}

func upsertRentalItems(ctx context.Context, tx pgx.Tx, tenantID, rentalID string, items []domain.RentalItem) error {
	for _, item := range items {
		_, err := tx.Exec(ctx, `
			INSERT INTO rental_items (
				id, tenant_id, rental_id, product_item_id, daily_rate, planned_days,
				actual_days, line_total, status, returned_at, return_condition,
				damage_assessment, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW())
		`,
			item.ID,
			tenantID,
			rentalID,
			item.ProductItemID,
			item.DailyRate,
			item.PlannedDays,
			item.ActualDays,
			item.LineTotal,
			string(item.Status),
			item.ReturnedAt,
			item.ReturnCondition,
			item.DamageAssessment,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertFeeLines(ctx context.Context, tx pgx.Tx, tenantID, rentalID string, feeLines []domain.FeeLine) error {
	for _, fee := range feeLines {
		_, err := tx.Exec(ctx, `
			INSERT INTO fee_lines (
				id, tenant_id, rental_id, rental_item_id, fee_type,
				amount, notes, created_by, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
			fee.ID,
			tenantID,
			rentalID,
			fee.RentalItemID,
			string(fee.FeeType),
			fee.Amount,
			fee.Notes,
			fee.CreatedBy,
			fee.CreatedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

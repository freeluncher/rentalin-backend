package usecase

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freeluncher/rentalin-backend/internal/domain"
	"github.com/freeluncher/rentalin-backend/internal/port"
)

type RentalWorkflowUsecase struct {
	inventoryRepo port.InventoryRepository
	rentalRepo    port.RentalRepository
	auditRepo     port.AuditRepository
}

func NewRentalWorkflowUsecase(inventoryRepo port.InventoryRepository, rentalRepo port.RentalRepository, auditRepo port.AuditRepository) *RentalWorkflowUsecase {
	return &RentalWorkflowUsecase{
		inventoryRepo: inventoryRepo,
		rentalRepo:    rentalRepo,
		auditRepo:     auditRepo,
	}
}

type CreateRentalItemInput struct {
	ProductItemID string  `json:"product_item_id"`
	DailyRate     float64 `json:"daily_rate"`
}

type CreateRentalInput struct {
	TenantID     string                  `json:"tenant_id"`
	CustomerName string                  `json:"customer_name"`
	StartAt      time.Time               `json:"start_at"`
	DueAt        time.Time               `json:"due_at"`
	CreatedBy    string                  `json:"created_by"`
	Items        []CreateRentalItemInput `json:"items"`
}

func (u *RentalWorkflowUsecase) CheckAvailability(ctx context.Context, tenantID string, itemIDs []string) ([]domain.ProductItem, error) {
	if tenantID == "" || len(itemIDs) == 0 {
		return nil, domain.ErrInvalidInput
	}

	items, err := u.inventoryRepo.GetByIDs(ctx, tenantID, itemIDs)
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

	return items, nil
}

func (u *RentalWorkflowUsecase) CreateRental(ctx context.Context, input CreateRentalInput) (domain.Rental, error) {
	if input.TenantID == "" || input.CustomerName == "" || input.StartAt.IsZero() || input.DueAt.IsZero() || len(input.Items) == 0 {
		return domain.Rental{}, domain.ErrInvalidInput
	}
	if !input.DueAt.After(input.StartAt) {
		return domain.Rental{}, domain.ErrInvalidInput
	}

	itemIDs := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		if item.ProductItemID == "" || item.DailyRate <= 0 {
			return domain.Rental{}, domain.ErrInvalidInput
		}
		itemIDs = append(itemIDs, item.ProductItemID)
	}

	_, err := u.inventoryRepo.ReserveAvailableItems(ctx, input.TenantID, itemIDs)
	if err != nil {
		return domain.Rental{}, err
	}

	plannedDays := int(math.Ceil(input.DueAt.Sub(input.StartAt).Hours() / 24))
	if plannedDays < 1 {
		plannedDays = 1
	}

	now := time.Now().UTC()
	rental := domain.Rental{
		ID:           uuid.NewString(),
		TenantID:     input.TenantID,
		CustomerName: input.CustomerName,
		StartAt:      input.StartAt.UTC(),
		DueAt:        input.DueAt.UTC(),
		Status:       domain.RentalStatusReserved,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	subtotal := 0.0
	rentalItems := make([]domain.RentalItem, 0, len(input.Items))
	for _, item := range input.Items {
		lineTotal := float64(plannedDays) * item.DailyRate
		subtotal += lineTotal
		rentalItems = append(rentalItems, domain.RentalItem{
			ID:            uuid.NewString(),
			ProductItemID: item.ProductItemID,
			DailyRate:     item.DailyRate,
			PlannedDays:   plannedDays,
			LineTotal:     lineTotal,
			Status:        domain.RentalItemStatusReserved,
		})
	}
	rental.RentalItems = rentalItems
	rental.Subtotal = subtotal
	rental.TotalFees = 0
	rental.GrandTotal = subtotal

	saved, err := u.rentalRepo.Create(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   input.TenantID,
		ActorUser:  input.CreatedBy,
		Action:     "rental.created",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"item_count": len(saved.RentalItems),
			"status":     saved.Status,
		},
	})

	return saved, nil
}

func (u *RentalWorkflowUsecase) CheckoutRental(ctx context.Context, tenantID, rentalID, actor string) (domain.Rental, error) {
	rental, err := u.rentalRepo.GetByID(ctx, tenantID, rentalID)
	if err != nil {
		return domain.Rental{}, err
	}
	if rental.Status != domain.RentalStatusReserved {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	itemIDs := make([]string, 0, len(rental.RentalItems))
	for _, item := range rental.RentalItems {
		itemIDs = append(itemIDs, item.ProductItemID)
	}
	if err := u.inventoryRepo.TransitionItemsStatus(ctx, tenantID, itemIDs, domain.ItemStatusReserved, domain.ItemStatusRented); err != nil {
		return domain.Rental{}, err
	}

	now := time.Now().UTC()
	for i := range rental.RentalItems {
		rental.RentalItems[i].Status = domain.RentalItemStatusRented
	}
	rental.Status = domain.RentalStatusActive
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   tenantID,
		ActorUser:  actor,
		Action:     "rental.checked_out",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
	})

	return saved, nil
}

type ReturnItemInput struct {
	ProductItemID string    `json:"product_item_id"`
	ReturnedAt    time.Time `json:"returned_at"`
	Condition     string    `json:"condition"`
	DamageCost    float64   `json:"damage_cost"`
}

type ProcessReturnInput struct {
	TenantID string            `json:"tenant_id"`
	RentalID string            `json:"rental_id"`
	Actor    string            `json:"actor"`
	Items    []ReturnItemInput `json:"items"`
}

func (u *RentalWorkflowUsecase) ProcessReturn(ctx context.Context, input ProcessReturnInput) (domain.Rental, error) {
	if input.TenantID == "" || input.RentalID == "" || len(input.Items) == 0 {
		return domain.Rental{}, domain.ErrInvalidInput
	}

	rental, err := u.rentalRepo.GetByID(ctx, input.TenantID, input.RentalID)
	if err != nil {
		return domain.Rental{}, err
	}
	if rental.Status != domain.RentalStatusActive && rental.Status != domain.RentalStatusPartiallyReturned {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	itemIndex := make(map[string]int)
	for i := range rental.RentalItems {
		itemIndex[rental.RentalItems[i].ProductItemID] = i
	}

	now := time.Now().UTC()
	for _, returned := range input.Items {
		idx, ok := itemIndex[returned.ProductItemID]
		if !ok {
			return domain.Rental{}, domain.ErrItemNotInRental
		}
		if rental.RentalItems[idx].Status == domain.RentalItemStatusReturned {
			return domain.Rental{}, domain.ErrReturnAlreadyHandled
		}

		returnedAt := returned.ReturnedAt.UTC()
		if returnedAt.IsZero() {
			returnedAt = now
		}

		actualDays := int(math.Ceil(returnedAt.Sub(rental.StartAt).Hours() / 24))
		if actualDays < 1 {
			actualDays = 1
		}
		rental.RentalItems[idx].ActualDays = actualDays
		rental.RentalItems[idx].ReturnedAt = &returnedAt
		rental.RentalItems[idx].ReturnCondition = strings.ToLower(strings.TrimSpace(returned.Condition))
		rental.RentalItems[idx].DamageAssessment = returned.DamageCost
		rental.RentalItems[idx].Status = domain.RentalItemStatusReturned

		if returnedAt.After(rental.DueAt) {
			lateDays := int(math.Ceil(returnedAt.Sub(rental.DueAt).Hours() / 24))
			if lateDays < 1 {
				lateDays = 1
			}
			lateFee := float64(lateDays) * rental.RentalItems[idx].DailyRate
			rental.FeeLines = append(rental.FeeLines, domain.FeeLine{
				ID:           uuid.NewString(),
				RentalID:     rental.ID,
				RentalItemID: rental.RentalItems[idx].ID,
				FeeType:      domain.FeeTypeLate,
				Amount:       lateFee,
				Notes:        "late return fee",
				CreatedBy:    input.Actor,
				CreatedAt:    now,
			})
		}

		if returned.DamageCost > 0 {
			rental.FeeLines = append(rental.FeeLines, domain.FeeLine{
				ID:           uuid.NewString(),
				RentalID:     rental.ID,
				RentalItemID: rental.RentalItems[idx].ID,
				FeeType:      domain.FeeTypeDamage,
				Amount:       returned.DamageCost,
				Notes:        "damage assessment fee",
				CreatedBy:    input.Actor,
				CreatedAt:    now,
			})
		}

		nextItemStatus := domain.ItemStatusAvailable
		if returned.DamageCost > 0 || strings.Contains(rental.RentalItems[idx].ReturnCondition, "damaged") {
			nextItemStatus = domain.ItemStatusMaintenance
		}
		if err := u.inventoryRepo.SetItemStatus(ctx, input.TenantID, returned.ProductItemID, nextItemStatus); err != nil {
			return domain.Rental{}, err
		}
	}

	allReturned := true
	var latestReturn *time.Time
	for i := range rental.RentalItems {
		if rental.RentalItems[i].Status != domain.RentalItemStatusReturned && rental.RentalItems[i].Status != domain.RentalItemStatusLost {
			allReturned = false
		}
		if rental.RentalItems[i].ReturnedAt != nil {
			if latestReturn == nil || rental.RentalItems[i].ReturnedAt.After(*latestReturn) {
				t := *rental.RentalItems[i].ReturnedAt
				latestReturn = &t
			}
		}
	}

	rental.Status = domain.RentalStatusPartiallyReturned
	if allReturned && latestReturn != nil {
		rental.ReturnedAt = latestReturn
	}

	totalFees := 0.0
	for _, fee := range rental.FeeLines {
		totalFees += fee.Amount
	}
	rental.TotalFees = totalFees
	rental.GrandTotal = rental.Subtotal + totalFees
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   input.TenantID,
		ActorUser:  input.Actor,
		Action:     "rental.item_returned",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"returned_item_count": len(input.Items),
			"status":              saved.Status,
		},
	})

	return saved, nil
}

func (u *RentalWorkflowUsecase) CloseRental(ctx context.Context, tenantID, rentalID, actor string) (domain.Rental, error) {
	rental, err := u.rentalRepo.GetByID(ctx, tenantID, rentalID)
	if err != nil {
		return domain.Rental{}, err
	}
	if rental.Status != domain.RentalStatusPartiallyReturned {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	for _, item := range rental.RentalItems {
		if item.Status != domain.RentalItemStatusReturned && item.Status != domain.RentalItemStatusLost {
			return domain.Rental{}, domain.ErrSettlementIncomplete
		}
	}

	now := time.Now().UTC()
	rental.Status = domain.RentalStatusCompleted
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   tenantID,
		ActorUser:  actor,
		Action:     "rental.closed",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
	})

	return saved, nil
}

func (u *RentalWorkflowUsecase) ListRentals(ctx context.Context, tenantID string) ([]domain.Rental, error) {
	if tenantID == "" {
		return nil, domain.ErrInvalidInput
	}
	return u.rentalRepo.ListByTenant(ctx, tenantID)
}

func (u *RentalWorkflowUsecase) GetRental(ctx context.Context, tenantID, rentalID string) (domain.Rental, error) {
	if tenantID == "" || rentalID == "" {
		return domain.Rental{}, domain.ErrInvalidInput
	}
	return u.rentalRepo.GetByID(ctx, tenantID, rentalID)
}

type ExtendRentalInput struct {
	TenantID string    `json:"tenant_id"`
	RentalID string    `json:"rental_id"`
	Actor    string    `json:"actor"`
	Reason   string    `json:"reason"`
	NewDueAt time.Time `json:"new_due_at"`
}

func (u *RentalWorkflowUsecase) ExtendRental(ctx context.Context, input ExtendRentalInput) (domain.Rental, error) {
	if input.TenantID == "" || input.RentalID == "" || input.NewDueAt.IsZero() {
		return domain.Rental{}, domain.ErrInvalidInput
	}

	rental, err := u.rentalRepo.GetByID(ctx, input.TenantID, input.RentalID)
	if err != nil {
		return domain.Rental{}, err
	}

	if rental.Status != domain.RentalStatusActive && rental.Status != domain.RentalStatusPartiallyReturned {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	if !input.NewDueAt.After(rental.DueAt) {
		return domain.Rental{}, domain.ErrExtensionInvalidDate
	}

	outstanding := make([]string, 0, len(rental.RentalItems))
	for _, item := range rental.RentalItems {
		if item.Status == domain.RentalItemStatusReserved || item.Status == domain.RentalItemStatusRented {
			outstanding = append(outstanding, item.ProductItemID)
		}
	}

	if len(outstanding) > 0 {
		conflict, err := u.rentalRepo.HasActiveScheduleConflict(ctx, input.TenantID, outstanding, rental.StartAt, input.NewDueAt, rental.ID)
		if err != nil {
			return domain.Rental{}, err
		}
		if conflict {
			return domain.Rental{}, domain.ErrExtensionConflict
		}
	}

	plannedDays := int(math.Ceil(input.NewDueAt.Sub(rental.StartAt).Hours() / 24))
	if plannedDays < 1 {
		plannedDays = 1
	}

	subtotal := 0.0
	for i := range rental.RentalItems {
		rental.RentalItems[i].PlannedDays = plannedDays
		rental.RentalItems[i].LineTotal = float64(plannedDays) * rental.RentalItems[i].DailyRate
		subtotal += rental.RentalItems[i].LineTotal
	}

	now := time.Now().UTC()
	rental.DueAt = input.NewDueAt.UTC()
	rental.Subtotal = subtotal
	rental.GrandTotal = rental.Subtotal + rental.TotalFees
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   input.TenantID,
		ActorUser:  input.Actor,
		Action:     "rental.extended",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"reason":      input.Reason,
			"new_due_at":  saved.DueAt,
			"item_count":  len(saved.RentalItems),
			"grand_total": saved.GrandTotal,
		},
	})

	return saved, nil
}

type CancelRentalInput struct {
	TenantID string `json:"tenant_id"`
	RentalID string `json:"rental_id"`
	Actor    string `json:"actor"`
	Reason   string `json:"reason"`
}

func (u *RentalWorkflowUsecase) CancelRental(ctx context.Context, input CancelRentalInput) (domain.Rental, error) {
	if input.TenantID == "" || input.RentalID == "" {
		return domain.Rental{}, domain.ErrInvalidInput
	}

	rental, err := u.rentalRepo.GetByID(ctx, input.TenantID, input.RentalID)
	if err != nil {
		return domain.Rental{}, err
	}

	if rental.Status != domain.RentalStatusDraft && rental.Status != domain.RentalStatusReserved {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	reservedItemIDs := make([]string, 0, len(rental.RentalItems))
	for i := range rental.RentalItems {
		if rental.RentalItems[i].Status == domain.RentalItemStatusReserved {
			reservedItemIDs = append(reservedItemIDs, rental.RentalItems[i].ProductItemID)
		}
	}

	if len(reservedItemIDs) > 0 {
		if err := u.inventoryRepo.TransitionItemsStatus(ctx, input.TenantID, reservedItemIDs, domain.ItemStatusReserved, domain.ItemStatusAvailable); err != nil {
			return domain.Rental{}, err
		}
	}

	now := time.Now().UTC()
	rental.Status = domain.RentalStatusCancelled
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   input.TenantID,
		ActorUser:  input.Actor,
		Action:     "rental.cancelled",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"reason": input.Reason,
		},
	})

	return saved, nil
}

type LostItemInput struct {
	ProductItemID string  `json:"product_item_id"`
	Compensation  float64 `json:"compensation"`
	Notes         string  `json:"notes"`
}

type MarkLostItemsInput struct {
	TenantID string          `json:"tenant_id"`
	RentalID string          `json:"rental_id"`
	Actor    string          `json:"actor"`
	Items    []LostItemInput `json:"items"`
}

func (u *RentalWorkflowUsecase) MarkLostItems(ctx context.Context, input MarkLostItemsInput) (domain.Rental, error) {
	if input.TenantID == "" || input.RentalID == "" || len(input.Items) == 0 {
		return domain.Rental{}, domain.ErrInvalidInput
	}

	rental, err := u.rentalRepo.GetByID(ctx, input.TenantID, input.RentalID)
	if err != nil {
		return domain.Rental{}, err
	}

	if rental.Status != domain.RentalStatusActive && rental.Status != domain.RentalStatusPartiallyReturned {
		return domain.Rental{}, domain.ErrRentalStatusInvalid
	}

	itemIndex := make(map[string]int, len(rental.RentalItems))
	for i := range rental.RentalItems {
		itemIndex[rental.RentalItems[i].ProductItemID] = i
	}

	now := time.Now().UTC()
	for _, lost := range input.Items {
		idx, ok := itemIndex[lost.ProductItemID]
		if !ok {
			return domain.Rental{}, domain.ErrItemNotInRental
		}

		if rental.RentalItems[idx].Status == domain.RentalItemStatusReturned || rental.RentalItems[idx].Status == domain.RentalItemStatusLost {
			return domain.Rental{}, domain.ErrRentalStatusInvalid
		}

		rental.RentalItems[idx].Status = domain.RentalItemStatusLost
		rental.RentalItems[idx].ReturnedAt = &now
		rental.RentalItems[idx].ReturnCondition = "lost"

		if err := u.inventoryRepo.SetItemStatus(ctx, input.TenantID, lost.ProductItemID, domain.ItemStatusLost); err != nil {
			return domain.Rental{}, err
		}

		if lost.Compensation > 0 {
			rental.FeeLines = append(rental.FeeLines, domain.FeeLine{
				ID:           uuid.NewString(),
				RentalID:     rental.ID,
				RentalItemID: rental.RentalItems[idx].ID,
				FeeType:      domain.FeeTypeOther,
				Amount:       lost.Compensation,
				Notes:        "lost item compensation: " + strings.TrimSpace(lost.Notes),
				CreatedBy:    input.Actor,
				CreatedAt:    now,
			})
		}
	}

	allSettled := true
	for _, item := range rental.RentalItems {
		if item.Status != domain.RentalItemStatusReturned && item.Status != domain.RentalItemStatusLost {
			allSettled = false
			break
		}
	}

	rental.Status = domain.RentalStatusPartiallyReturned
	if allSettled {
		rental.ReturnedAt = &now
	}

	totalFees := 0.0
	for _, fee := range rental.FeeLines {
		totalFees += fee.Amount
	}
	rental.TotalFees = totalFees
	rental.GrandTotal = rental.Subtotal + totalFees
	rental.UpdatedAt = now

	saved, err := u.rentalRepo.Update(ctx, rental)
	if err != nil {
		return domain.Rental{}, err
	}

	_ = u.auditRepo.Append(ctx, port.AuditLogEntry{
		TenantID:   input.TenantID,
		ActorUser:  input.Actor,
		Action:     "rental.item_lost",
		Entity:     "rental",
		EntityID:   saved.ID,
		OccurredAt: now,
		Payload: map[string]any{
			"lost_item_count": len(input.Items),
		},
	})

	return saved, nil
}

package domain

import (
	"errors"
	"time"
)

type ItemAvailabilityStatus string

const (
	ItemStatusAvailable   ItemAvailabilityStatus = "available"
	ItemStatusReserved    ItemAvailabilityStatus = "reserved"
	ItemStatusRented      ItemAvailabilityStatus = "rented"
	ItemStatusMaintenance ItemAvailabilityStatus = "maintenance"
	ItemStatusLost        ItemAvailabilityStatus = "lost"
)

type RentalStatus string

const (
	RentalStatusDraft             RentalStatus = "draft"
	RentalStatusReserved          RentalStatus = "reserved"
	RentalStatusActive            RentalStatus = "active"
	RentalStatusPartiallyReturned RentalStatus = "partially_returned"
	RentalStatusCompleted         RentalStatus = "completed"
	RentalStatusCancelled         RentalStatus = "cancelled"
)

type RentalItemStatus string

const (
	RentalItemStatusReserved RentalItemStatus = "reserved"
	RentalItemStatusRented   RentalItemStatus = "rented"
	RentalItemStatusReturned RentalItemStatus = "returned"
	RentalItemStatusLost     RentalItemStatus = "lost"
)

type FeeType string

const (
	FeeTypeLate   FeeType = "late"
	FeeTypeDamage FeeType = "damage"
	FeeTypeOther  FeeType = "other"
)

type ProductItem struct {
	ID                 string                 `json:"id"`
	TenantID           string                 `json:"tenant_id"`
	ProductID          string                 `json:"product_id"`
	SerialNumber       string                 `json:"serial_number"`
	ConditionStatus    string                 `json:"condition_status"`
	AvailabilityStatus ItemAvailabilityStatus `json:"availability_status"`
}

type RentalItem struct {
	ID               string           `json:"id"`
	ProductItemID    string           `json:"product_item_id"`
	DailyRate        float64          `json:"daily_rate"`
	PlannedDays      int              `json:"planned_days"`
	ActualDays       int              `json:"actual_days"`
	LineTotal        float64          `json:"line_total"`
	Status           RentalItemStatus `json:"status"`
	ReturnedAt       *time.Time       `json:"returned_at,omitempty"`
	ReturnCondition  string           `json:"return_condition,omitempty"`
	DamageAssessment float64          `json:"damage_assessment,omitempty"`
}

type FeeLine struct {
	ID           string    `json:"id"`
	RentalID     string    `json:"rental_id"`
	RentalItemID string    `json:"rental_item_id,omitempty"`
	FeeType      FeeType   `json:"fee_type"`
	Amount       float64   `json:"amount"`
	Notes        string    `json:"notes"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type Rental struct {
	ID           string       `json:"id"`
	TenantID     string       `json:"tenant_id"`
	CustomerName string       `json:"customer_name"`
	StartAt      time.Time    `json:"start_at"`
	DueAt        time.Time    `json:"due_at"`
	ReturnedAt   *time.Time   `json:"returned_at,omitempty"`
	Status       RentalStatus `json:"status"`
	RentalItems  []RentalItem `json:"rental_items"`
	FeeLines     []FeeLine    `json:"fee_lines"`
	Subtotal     float64      `json:"subtotal"`
	TotalFees    float64      `json:"total_fees"`
	GrandTotal   float64      `json:"grand_total"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

var (
	ErrInvalidInput         = errors.New("invalid input")
	ErrRentalNotFound       = errors.New("rental not found")
	ErrRentalStatusInvalid  = errors.New("rental status invalid")
	ErrItemUnavailable      = errors.New("item unavailable")
	ErrItemNotInRental      = errors.New("item not in rental")
	ErrReturnAlreadyHandled = errors.New("return already processed")
	ErrSettlementIncomplete = errors.New("settlement incomplete")
)

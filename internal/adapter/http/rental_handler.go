package http

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/freeluncher/rentalin-backend/internal/domain"
	"github.com/freeluncher/rentalin-backend/internal/usecase"
)

type RentalHandler struct {
	workflow *usecase.RentalWorkflowUsecase
}

func NewRentalHandler(workflow *usecase.RentalWorkflowUsecase) *RentalHandler {
	return &RentalHandler{workflow: workflow}
}

type createRentalRequest struct {
	TenantID     string `json:"tenant_id"`
	CustomerName string `json:"customer_name"`
	StartAt      string `json:"start_at"`
	DueAt        string `json:"due_at"`
	CreatedBy    string `json:"created_by"`
	Items        []struct {
		ProductItemID string  `json:"product_item_id"`
		DailyRate     float64 `json:"daily_rate"`
	} `json:"items"`
}

type checkoutRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
}

type returnRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
	Items    []struct {
		ProductItemID string  `json:"product_item_id"`
		ReturnedAt    string  `json:"returned_at"`
		Condition     string  `json:"condition"`
		DamageCost    float64 `json:"damage_cost"`
	} `json:"items"`
}

type closeRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
}

type extensionRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
	Reason   string `json:"reason"`
	NewDueAt string `json:"new_due_at"`
}

type cancelRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
	Reason   string `json:"reason"`
}

type lostItemsRequest struct {
	TenantID string `json:"tenant_id"`
	Actor    string `json:"actor"`
	Items    []struct {
		ProductItemID string  `json:"product_item_id"`
		Compensation  float64 `json:"compensation"`
		Notes         string  `json:"notes"`
	} `json:"items"`
}

func (h *RentalHandler) RegisterRoutes(router fiber.Router) {
	router.Get("/availability", h.CheckAvailability)
	router.Post("/rentals", h.CreateRental)
	router.Get("/rentals", h.ListRentals)
	router.Get("/rentals/:id", h.GetRental)
	router.Post("/rentals/:id/checkout", h.CheckoutRental)
	router.Post("/rentals/:id/extensions", h.ExtendRental)
	router.Post("/rentals/:id/cancel", h.CancelRental)
	router.Post("/rentals/:id/returns", h.ProcessReturn)
	router.Post("/rentals/:id/lost-items", h.MarkLostItems)
	router.Post("/rentals/:id/close", h.CloseRental)
}

func (h *RentalHandler) CheckAvailability(c *fiber.Ctx) error {
	tenantID := c.Query("tenant_id")
	itemIDsRaw := c.Query("item_ids")
	if tenantID == "" || itemIDsRaw == "" {
		return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", "tenant_id and item_ids are required")
	}

	parts := strings.Split(itemIDsRaw, ",")
	itemIDs := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			itemIDs = append(itemIDs, trimmed)
		}
	}

	items, err := h.workflow.CheckAvailability(c.UserContext(), tenantID, itemIDs)
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant_id": tenantID,
			"available": true,
			"items":     items,
		},
	})
}

func (h *RentalHandler) CreateRental(c *fiber.Ctx) error {
	var req createRentalRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", "start_at must be RFC3339")
	}
	dueAt, err := time.Parse(time.RFC3339, req.DueAt)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", "due_at must be RFC3339")
	}

	items := make([]usecase.CreateRentalItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, usecase.CreateRentalItemInput{
			ProductItemID: item.ProductItemID,
			DailyRate:     item.DailyRate,
		})
	}

	rental, err := h.workflow.CreateRental(c.UserContext(), usecase.CreateRentalInput{
		TenantID:     req.TenantID,
		CustomerName: req.CustomerName,
		StartAt:      startAt,
		DueAt:        dueAt,
		CreatedBy:    req.CreatedBy,
		Items:        items,
	})
	if err != nil {
		return respondDomainError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) ListRentals(c *fiber.Ctx) error {
	tenantID := c.Query("tenant_id")
	rentals, err := h.workflow.ListRentals(c.UserContext(), tenantID)
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rentals})
}

func (h *RentalHandler) GetRental(c *fiber.Ctx) error {
	tenantID := c.Query("tenant_id")
	rentalID := c.Params("id")
	rental, err := h.workflow.GetRental(c.UserContext(), tenantID, rentalID)
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) CheckoutRental(c *fiber.Ctx) error {
	var req checkoutRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	rental, err := h.workflow.CheckoutRental(c.UserContext(), req.TenantID, c.Params("id"), req.Actor)
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) ProcessReturn(c *fiber.Ctx) error {
	var req returnRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	items := make([]usecase.ReturnItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		returnedAt := time.Now().UTC()
		if strings.TrimSpace(item.ReturnedAt) != "" {
			parsed, err := time.Parse(time.RFC3339, item.ReturnedAt)
			if err != nil {
				return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", "returned_at must be RFC3339")
			}
			returnedAt = parsed
		}
		items = append(items, usecase.ReturnItemInput{
			ProductItemID: item.ProductItemID,
			ReturnedAt:    returnedAt,
			Condition:     item.Condition,
			DamageCost:    item.DamageCost,
		})
	}

	rental, err := h.workflow.ProcessReturn(c.UserContext(), usecase.ProcessReturnInput{
		TenantID: req.TenantID,
		RentalID: c.Params("id"),
		Actor:    req.Actor,
		Items:    items,
	})
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) ExtendRental(c *fiber.Ctx) error {
	var req extensionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	newDueAt, err := time.Parse(time.RFC3339, req.NewDueAt)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", "new_due_at must be RFC3339")
	}

	rental, err := h.workflow.ExtendRental(c.UserContext(), usecase.ExtendRentalInput{
		TenantID: req.TenantID,
		RentalID: c.Params("id"),
		Actor:    req.Actor,
		Reason:   req.Reason,
		NewDueAt: newDueAt,
	})
	if err != nil {
		return respondDomainError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) CancelRental(c *fiber.Ctx) error {
	var req cancelRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	rental, err := h.workflow.CancelRental(c.UserContext(), usecase.CancelRentalInput{
		TenantID: req.TenantID,
		RentalID: c.Params("id"),
		Actor:    req.Actor,
		Reason:   req.Reason,
	})
	if err != nil {
		return respondDomainError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) MarkLostItems(c *fiber.Ctx) error {
	var req lostItemsRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	items := make([]usecase.LostItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, usecase.LostItemInput{
			ProductItemID: item.ProductItemID,
			Compensation:  item.Compensation,
			Notes:         item.Notes,
		})
	}

	rental, err := h.workflow.MarkLostItems(c.UserContext(), usecase.MarkLostItemsInput{
		TenantID: req.TenantID,
		RentalID: c.Params("id"),
		Actor:    req.Actor,
		Items:    items,
	})
	if err != nil {
		return respondDomainError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func (h *RentalHandler) CloseRental(c *fiber.Ctx) error {
	var req closeRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
	}

	rental, err := h.workflow.CloseRental(c.UserContext(), req.TenantID, c.Params("id"), req.Actor)
	if err != nil {
		return respondDomainError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": rental})
}

func respondDomainError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return respondError(c, fiber.StatusBadRequest, "INVALID_INPUT", err.Error())
	case errors.Is(err, domain.ErrItemUnavailable):
		return respondError(c, fiber.StatusConflict, "ITEM_UNAVAILABLE", err.Error())
	case errors.Is(err, domain.ErrRentalNotFound):
		return respondError(c, fiber.StatusNotFound, "RENTAL_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrRentalStatusInvalid):
		return respondError(c, fiber.StatusConflict, "RENTAL_STATUS_INVALID", err.Error())
	case errors.Is(err, domain.ErrItemNotInRental):
		return respondError(c, fiber.StatusBadRequest, "ITEM_NOT_IN_RENTAL", err.Error())
	case errors.Is(err, domain.ErrReturnAlreadyHandled):
		return respondError(c, fiber.StatusConflict, "RETURN_ALREADY_PROCESSED", err.Error())
	case errors.Is(err, domain.ErrSettlementIncomplete):
		return respondError(c, fiber.StatusConflict, "SETTLEMENT_INCOMPLETE", err.Error())
	case errors.Is(err, domain.ErrExtensionConflict):
		return respondError(c, fiber.StatusConflict, "EXTENSION_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrExtensionInvalidDate):
		return respondError(c, fiber.StatusBadRequest, "EXTENSION_INVALID_DATE", err.Error())
	default:
		return respondError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

func respondError(c *fiber.Ctx, httpStatus int, code, message string) error {
	return c.Status(httpStatus).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	})
}

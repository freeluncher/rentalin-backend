package app

import (
	"fmt"

	httpAdapter "github.com/freeluncher/rentalin-backend/internal/adapter/http"
	"github.com/freeluncher/rentalin-backend/internal/adapter/memory"
	"github.com/freeluncher/rentalin-backend/internal/domain"
	"github.com/freeluncher/rentalin-backend/internal/platform/config"
	"github.com/freeluncher/rentalin-backend/internal/usecase"
)

func Run() error {
	cfg := config.Load()

	inventoryRepo := memory.NewInventoryRepository(seedItems(cfg.SeedTenantID))
	rentalRepo := memory.NewRentalRepository()
	auditRepo := memory.NewAuditRepository()

	workflow := usecase.NewRentalWorkflowUsecase(inventoryRepo, rentalRepo, auditRepo)
	rentalHandler := httpAdapter.NewRentalHandler(workflow)
	app := httpAdapter.NewRouter(rentalHandler)

	fmt.Printf("server listening on :%s\n", cfg.Port)
	return app.Listen(":" + cfg.Port)
}

func seedItems(tenantID string) []domain.ProductItem {
	return []domain.ProductItem{
		{
			ID:                 "item-001",
			TenantID:           tenantID,
			ProductID:          "product-camera-a",
			SerialNumber:       "CAM-A-001",
			ConditionStatus:    "good",
			AvailabilityStatus: domain.ItemStatusAvailable,
		},
		{
			ID:                 "item-002",
			TenantID:           tenantID,
			ProductID:          "product-camera-a",
			SerialNumber:       "CAM-A-002",
			ConditionStatus:    "good",
			AvailabilityStatus: domain.ItemStatusAvailable,
		},
		{
			ID:                 "item-003",
			TenantID:           tenantID,
			ProductID:          "product-light-a",
			SerialNumber:       "LGT-A-001",
			ConditionStatus:    "good",
			AvailabilityStatus: domain.ItemStatusAvailable,
		},
	}
}

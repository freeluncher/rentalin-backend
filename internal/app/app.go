package app

import (
	"context"
	"fmt"

	httpAdapter "github.com/freeluncher/rentalin-backend/internal/adapter/http"
	"github.com/freeluncher/rentalin-backend/internal/adapter/memory"
	pgAdapter "github.com/freeluncher/rentalin-backend/internal/adapter/postgres"
	"github.com/freeluncher/rentalin-backend/internal/domain"
	"github.com/freeluncher/rentalin-backend/internal/platform/config"
	pgPlatform "github.com/freeluncher/rentalin-backend/internal/platform/postgres"
	"github.com/freeluncher/rentalin-backend/internal/port"
	"github.com/freeluncher/rentalin-backend/internal/usecase"
)

func Run() error {
	cfg := config.Load()

	var inventoryRepo port.InventoryRepository
	var rentalRepo port.RentalRepository
	var auditRepo port.AuditRepository

	if cfg.DBURL != "" {
		pool, err := pgPlatform.NewPool(context.Background(), cfg.DBURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		inventoryRepo = pgAdapter.NewInventoryRepository(pool)
		rentalRepo = pgAdapter.NewRentalRepository(pool)
		auditRepo = pgAdapter.NewAuditRepository(pool)
	} else {
		inventoryRepo = memory.NewInventoryRepository(seedItems(cfg.SeedTenantID))
		rentalRepo = memory.NewRentalRepository()
		auditRepo = memory.NewAuditRepository()
	}

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

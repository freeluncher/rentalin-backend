package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func NewRouter(rentalHandler *RentalHandler) *fiber.App {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Rentalin Backend API is running",
			"status":  "ok",
		})
	})

	v1 := app.Group("/v1")
	rentalHandler.RegisterRoutes(v1)

	return app
}

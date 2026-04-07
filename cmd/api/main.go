package main

import (
	"log"

	"github.com/freeluncher/rentalin-backend/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

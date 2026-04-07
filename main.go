package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Global variable untuk database
var DB *gorm.DB

// Struktur Model contoh (Tabel Units untuk Rental PS)
type Unit struct {
	gorm.Model
	Name     string `json:"name"`
	Type     string `json:"type"`      // misal: PS5, PS4
	PriceDay int    `json:"price_day"` // harga per hari
	Status   string `json:"status"`    // ready, rented, maintenance
}

func initDB() {
	// 1. Load file .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// 2. Ambil URL Database
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("DB_URL is empty in .env file")
	}
	fmt.Println("Connecting to:", dsn) // Cek apakah URL-nya sudah benar di terminal
	// 3. Koneksi ke Supabase (Postgres)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal koneksi ke database: ", err)
	}

	fmt.Println("✅ Berhasil terhubung ke Supabase!")

	// 4. Auto Migrate (Membuat tabel otomatis berdasarkan struct Unit)
	db.AutoMigrate(&Unit{})

	DB = db
}

func main() {
	// Inisialisasi Database
	initDB()

	// Inisialisasi Fiber App
	app := fiber.New()

	// Middleware Logger untuk melihat log request di terminal
	app.Use(logger.New())

	// Route: Health Check
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Backend Rentalin API is running!",
			"status":  "connected",
		})
	})

	// Route: Ambil Semua Unit PS (Contoh API)
	app.Get("/units", func(c *fiber.Ctx) error {
		var units []Unit
		DB.Find(&units)
		return c.JSON(units)
	})

	// Ambil Port dari .env
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Jalankan Server
	fmt.Printf("🚀 Server jalan di port %s\n", port)
	log.Fatal(app.Listen(":" + port))
}

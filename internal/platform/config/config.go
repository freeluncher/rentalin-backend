package config

import "os"

type Config struct {
	Port         string
	SeedTenantID string
	DBURL        string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	seedTenantID := os.Getenv("SEED_TENANT_ID")
	if seedTenantID == "" {
		seedTenantID = "tenant-demo"
	}

	dbURL := os.Getenv("DB_URL")

	return Config{
		Port:         port,
		SeedTenantID: seedTenantID,
		DBURL:        dbURL,
	}
}

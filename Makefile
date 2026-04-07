MIGRATE := go run -tags "postgres" github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3
MIGRATIONS_DIR := migrations
DOWN_STEPS ?= 1
EXAMPLE_DB_URL := postgres://user:pass@localhost:5432/rentalin?sslmode=disable

.PHONY: migrate-up migrate-down migrate-down-all migrate-version migrate-force check-db-url

check-db-url:
	$(if $(strip $(DB_URL)),,$(error DB_URL is required. Example: make migrate-up DB_URL="$(EXAMPLE_DB_URL)"))

migrate-up: check-db-url
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

migrate-down: check-db-url
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down $(DOWN_STEPS)

migrate-down-all: check-db-url
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down -all

migrate-version: check-db-url
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

migrate-force: check-db-url
	$(if $(strip $(VERSION)),,$(error VERSION is required. Example: make migrate-force DB_URL="$(EXAMPLE_DB_URL)" VERSION=202604080002))
	@$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" force $(VERSION)

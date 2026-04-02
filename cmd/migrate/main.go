package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/dbmigrate"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is required")
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}

	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := dbmigrate.Run(db, migrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	log.Printf("migrations applied successfully from %s", migrationsDir)
}

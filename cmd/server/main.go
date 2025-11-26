package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"Shopify-GreenMoney-Lockout/internal"
)

func main() {
	// 1. Get DB URL (env or hardcoded)
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:GoComets321!@@localhost:5432/Shopify_GreenMoney_Lockout?sslmode=disable"
	}

	// 2. Open DB
	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	// 3. Set up HTTP routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// NEW: Shopify orders/create webhook
	mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandler(db))

	mux.HandleFunc("/green/ipn", internal.GreenIPNHandler(db))

	addr := ":8081" // or :8080 if it's free again
	log.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Get DB URL (env or hardcoded)
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatalf("Could not retrive DB URL from .env")
	}

	// 2. Open DB
	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	shopifyStoreDomain := os.Getenv("SHOPIFY_STORE_DOMAIN")
	shopifyAccessToken := os.Getenv("SHOPIFY_ACCESS_TOKEN")
	shopifyAPIVersion := "2024-10" // or whatever version your app is set to

	if shopifyStoreDomain == "" || shopifyAccessToken == "" {
		log.Println("WARNING: SHOPIFY_STORE_DOMAIN or SHOPIFY_ACCESS_TOKEN not set; Shopify calls will fail")
	}

	shopifyClient := internal.NewShopifyClient(shopifyStoreDomain, shopifyAccessToken, shopifyAPIVersion)
	greenClient := internal.NewGreenClientFromEnv()
	// Start background poller to check invoice_sent payments with Green.
	// For development, we can poll every 60 seconds. For production you might
	// stretch this to 10-30 minutes depending on how fast you need updates.
	ctx := context.Background()
	internal.StartGreenPoller(ctx, db, greenClient, shopifyClient, 10*time.Second)

	// 3. Set up HTTP routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// NEW: Shopify orders/create webhook
	mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandler(db, greenClient))

	mux.HandleFunc("/green/ipn", internal.GreenIPNHandler(db, shopifyClient, greenClient))

	mux.HandleFunc("/debug/green/unseen", func(w http.ResponseWriter, r *http.Request) {
		res, err := greenClient.UnseenNotifications(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(res)
	})

	addr := ":8081" // or :8080 if it's free again
	log.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

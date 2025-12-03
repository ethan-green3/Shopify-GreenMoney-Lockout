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
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatalf("Could not retrive DB URL from .env")
	}

	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	if os.Getenv("SHOPIFY_STORE_DOMAIN") == "" || os.Getenv("SHOPIFY_ACCESS_TOKEN") == "" {
		log.Fatal("WARNING: SHOPIFY_STORE_DOMAIN or SHOPIFY_ACCESS_TOKEN not set; Shopify calls will fail")
	}

	shopifyClient := internal.NewShopifyClient(os.Getenv("SHOPIFY_STORE_DOMAIN"), os.Getenv("SHOPIFY_ACCESS_TOKEN"), os.Getenv("SHOPIFY_API_VERSION"))
	greenClient := internal.NewGreenClientFromEnv()

	ctx := context.Background()
	internal.StartGreenPoller(ctx, db, greenClient, shopifyClient, 1*time.Minute)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

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

	addr := ":8081"
	log.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

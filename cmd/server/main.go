package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/email"
	"Shopify-GreenMoney-Lockout/internal/moneyeu"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	testMode := os.Getenv("TEST_MODE")
	if testMode == "true" {
		log.Println("IN TESTING MODE, USING SANDBOX MONEYEU VARIABLES, CHECK CALLS BELOW TO MAKE SURE")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatalf("Could not retrive DB URL from .env")
	}
	smtp_port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		log.Fatalf("Could not retrieve SMTP Port from .env")
	}
	smtpCfg := email.SMTPConfig{
		Host: os.Getenv("SMTP_HOST"),
		Port: smtp_port,
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: os.Getenv("SMTP_FROM"),
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
	moneyClient := moneyeu.NewClient(
		os.Getenv("MONEYEU_BASE_URL"),
		os.Getenv("MONEYEU_SANDBOX_API_KEY"),
		os.Getenv("MONEYEU_SANDBOX_SECRET"),
	)
	moneySvc := &moneyeu.Service{
		DB:     db,
		Client: moneyClient,
		SMTP:   smtpCfg,
	}

	ctx := context.Background()
	internal.StartGreenPoller(ctx, db, greenClient, shopifyClient, 1*time.Minute)

	if os.Getenv("SEND_TEST_EMAIL") == "1" {
		err = email.Send(smtpCfg, "ethangreen2000@yahoo.com", "SMTP Test", "Hi Ethan, the SMTP Server for Lockout Supplements is up and running now")
		if err != nil {
			log.Fatalf("email send failed: %v", err)
		}
		log.Println("Test email sent successfully")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandler(db, greenClient, moneySvc))

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

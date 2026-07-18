package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/moneyeu"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func main() {
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		TimeFormat: time.Kitchen,
	})))

	godotenv.Load()
	testMode := os.Getenv("TEST_MODE")
	if testMode == "true" {
		log.Println("IN TESTING MODE, USING SANDBOX MONEYEU VARIABLES, CHECK CALLS BELOW TO MAKE SURE")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatalf("Could not retrive DB URL from .env")
	}
	// Local checkout-link email is currently disabled because MoneyEU hosted
	// checkout emails the link to the customer. Leave this SMTP wiring here as
	// a quick re-enable path if first-party email delivery is needed again.
	/*
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
	*/

	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	shopifyRegistry, err := internal.NewShopifyClientRegistryFromEnv()
	if err != nil {
		log.Fatalf("failed to load Shopify config: %v", err)
	}
	if !shopifyRegistry.HasAny() {
		log.Fatal("WARNING: no Shopify store config found; set SHOPIFY_STORE_DOMAIN/SHOPIFY_ACCESS_TOKEN or SHOPIFY_STORE_CONFIGS")
	}
	greenClient := internal.NewGreenClientFromEnv()
	moneyClient, err := moneyeu.NewClient(
		os.Getenv("MONEYEU_BASE_URL"),
		os.Getenv("MONEYEU_LIVE_API_KEY"),
		os.Getenv("MONEYEU_LIVE_SECRET"),
	)
	if err != nil {
		log.Fatalf("Missing env variables: %s", err)
	}
	moneyClient.ProcessPaymentURL = os.Getenv("MONEYEU_PROCESS_PAYMENT_URL")
	if processPaymentPath := os.Getenv("MONEYEU_PROCESS_PAYMENT_PATH"); processPaymentPath != "" {
		moneyClient.Path = moneyeu.NormalizePath(processPaymentPath)
	}
	moneySvc := &moneyeu.Service{
		DB:                     db,
		Client:                 moneyClient,
		ReturnURL:              os.Getenv("MONEYEU_RETURN_URL"),
		CallbackURL:            os.Getenv("MONEYEU_CALLBACK_URL"),
		MerchantName:           os.Getenv("MONEYEU_MERCHANT_NAME"),
		MerchantTerminalID:     os.Getenv("MONEYEU_MERCHANT_TERMINAL_ID"),
		StoreFrontURL:          os.Getenv("MONEYEU_STOREFRONT_URL"),
		CustomerIPAddress:      os.Getenv("MONEYEU_CUSTOMER_IP_ADDRESS"),
		CustomerUserAgent:      os.Getenv("MONEYEU_CUSTOMER_USER_AGENT"),
		IncludeReferenceFields: os.Getenv("MONEYEU_INCLUDE_REFERENCE_FIELDS") == "1",
	}

	ctx := context.Background()
	internal.StartGreenPoller(ctx, db, greenClient, shopifyRegistry, 1*time.Minute)

	/*
		if os.Getenv("SEND_TEST_EMAIL") == "1" {
			err = email.Send(smtpCfg, "ethangreen2000@yahoo.com", "SMTP Test", "Hi Ethan, the SMTP Server for Lockout Supplements is up and running now")
			if err != nil {
				log.Fatalf("email send failed: %v", err)
			}
			log.Println("Test email sent successfully")
		}
	*/

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/webhooks/moneyeu", moneyeu.MoneyEUWebhookHandler(db, moneyeuShopifyResolver{registry: shopifyRegistry}))

	mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandler(db, greenClient, moneySvc))

	mux.HandleFunc("/green/ipn", internal.GreenIPNHandler(db, shopifyRegistry, greenClient))

	mux.HandleFunc("/debug/green/unseen", func(w http.ResponseWriter, r *http.Request) {
		res, err := greenClient.UnseenNotifications(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(res)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081" // local fallback
	}

	addr := ":" + port
	log.Printf("Starting server on %s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}

}

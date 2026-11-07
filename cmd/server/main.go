package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/burner"
	"Shopify-GreenMoney-Lockout/internal/email"
	"Shopify-GreenMoney-Lockout/internal/moneyeu"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	smtpPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	smtpCfg := email.SMTPConfig{Host: os.Getenv("SMTP_HOST"), Port: smtpPort, User: os.Getenv("SMTP_USER"), Pass: os.Getenv("SMTP_PASS"), From: os.Getenv("SMTP_FROM")}

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
	webhookVerifier := shopifyRegistry.WebhookSecrets()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SHOPIFY_WEBHOOK_ENFORCE")), "0") && strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development") {
		webhookVerifier = nil
	}
	var burnerSvc *burner.Service
	if strings.EqualFold(strings.TrimSpace(os.Getenv("BURNER_ENABLED")), "1") || strings.EqualFold(strings.TrimSpace(os.Getenv("BURNER_ENABLED")), "true") {
		version := strings.TrimSpace(os.Getenv("BURNER_API_VERSION"))
		if version == "" {
			log.Fatal("BURNER_API_VERSION is required when Burner is enabled")
		}
		if strings.TrimSpace(os.Getenv("BURNER_API_KEY")) == "" || strings.TrimSpace(os.Getenv("BURNER_WEBHOOK_SECRET")) == "" {
			log.Fatal("BURNER_API_KEY and BURNER_WEBHOOK_SECRET are required when Burner is enabled")
		}
		if !shopifyRegistry.WebhookSecrets().HasAny() {
			log.Fatal("at least one Shopify webhook secret is required when Burner is enabled")
		}
		if smtpCfg.Host == "" || smtpCfg.Port == 0 || smtpCfg.From == "" {
			log.Fatal("SMTP_HOST, SMTP_PORT, and SMTP_FROM are required when Burner is enabled")
		}
		base := os.Getenv("BURNER_BASE_URL")
		if base == "" {
			base = "https://merchant.burner.pro"
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(base)), "https://") && os.Getenv("BURNER_ALLOW_INSECURE_HTTP") != "1" {
			log.Fatal("BURNER_BASE_URL must use HTTPS")
		}
		checkout := os.Getenv("BURNER_CHECKOUT_BASE_URL")
		if checkout == "" {
			checkout = "https://pay.burner.pro"
		}
		burnerSvc = &burner.Service{Store: &burner.Store{DB: db}, Client: &burner.Client{BaseURL: base, APIKey: os.Getenv("BURNER_API_KEY"), APIVersion: version}, CheckoutBaseURL: checkout, Email: burnerEmailSender{cfg: smtpCfg}, Shopify: burnerShopifyResolver{registry: shopifyRegistry}}
		go burnerSvc.Run(ctx)
	}
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

	if burnerSvc != nil {
		mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandlerWithBurner(db, greenClient, moneySvc, burnerSvc, webhookVerifier))
	} else {
		mux.HandleFunc("/webhooks/shopify/orders-create", internal.ShopifyOrderCreateHandlerWithAuth(db, greenClient, moneySvc, webhookVerifier))
	}
	if burnerSvc != nil {
		mux.HandleFunc("/webhooks/burner", burner.WebhookHandler(burnerSvc.Store, os.Getenv("BURNER_WEBHOOK_SECRET"), time.Now))
	}

	mux.HandleFunc("/green/ipn", internal.GreenIPNHandlerSafe(db, shopifyRegistry, greenClient))

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

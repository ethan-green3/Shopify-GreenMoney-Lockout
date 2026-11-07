package main

import (
	"Shopify-GreenMoney-Lockout/internal"
	"Shopify-GreenMoney-Lockout/internal/burner"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "resend-burner-link" {
		log.Fatal("usage: paymentops resend-burner-link --shop SHOP_DOMAIN --order ORDER_ID")
	}
	fs := flag.NewFlagSet("resend-burner-link", flag.ExitOnError)
	shop := fs.String("shop", "", "Shopify shop domain")
	order := fs.Int64("order", 0, "Shopify numeric order ID")
	_ = fs.Parse(os.Args[2:])
	if *shop == "" || *order <= 0 {
		log.Fatal("--shop and a positive --order are required")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is required")
	}
	db, err := internal.OpenDB(dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	var paymentID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM burner_payments WHERE shop_domain=$1 AND shopify_order_id=$2`, *shop, *order).Scan(&paymentID); err != nil {
		log.Fatalf("Burner payment not found: %v", err)
	}
	store := &burner.Store{DB: db}
	p, err := store.GetPayment(ctx, paymentID)
	if err != nil {
		log.Fatal(err)
	}
	if !p.InvoiceID.Valid || p.CheckoutURL == "" {
		log.Fatal("cannot resend: Burner invoice has not been created")
	}
	if p.InvoiceStatus == burner.StatusExpired || p.InvoiceStatus == burner.StatusRefunded || p.InvoiceStatus == burner.StatusFailed {
		log.Fatalf("cannot resend: invoice status is %s", p.InvoiceStatus)
	}
	if err := store.Enqueue(ctx, p.ID, burner.JobSendCheckoutEmail, time.Now(), map[string]string{"reason": "operator_resend", "recipient": p.CustomerEmail}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("queued resend for payment %s (recipient %s)\n", strconv.FormatInt(p.ID, 10), p.CustomerEmail)
}

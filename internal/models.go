package internal

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// Status constants for green_payments.current_status.
const (
	StatusInvoiceSent = "invoice_sent"
	StatusCleared     = "cleared"
)

// GreenPayment matches the green_payments table structure (simplified for now).
type GreenPayment struct {
	ID               int64
	ShopifyOrderID   int64
	ShopifyOrderName string
	Amount           float64
	Currency         string

	InvoiceID     string
	GreenCheckID  string
	CurrentStatus string
	IsCleared     bool

	ShopifyMarkedPaidAt *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastStatusAt        time.Time
}

// InsertGreenPayment inserts a new row into the green_payments table.
// For now we fake InvoiceID and CheckID (we'll replace with real Green calls later).
func InsertGreenPayment(db *sql.DB, order ShopifyOrder) error {
	// Convert total_price (string) to float64
	amount, err := strconv.ParseFloat(order.TotalPrice, 64)
	if err != nil {
		return fmt.Errorf("invalid total_price %q: %w", order.TotalPrice, err)
	}

	// Temporary fake IDs for POC
	invoiceID := "INV-TEST"
	checkID := "CHK-TEST"

	query := `
		INSERT INTO green_payments (
			shopify_order_id,
			shopify_order_name,
			amount,
			currency,
			invoice_id,
			green_check_id,
			current_status,
			is_cleared,
			created_at,
			updated_at,
			last_status_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE, NOW(), NOW(), NOW())
	`

	_, err = db.Exec(
		query,
		order.ID,
		order.Name,
		amount,
		order.Currency,
		invoiceID,
		checkID,
		StatusInvoiceSent,
	)
	if err != nil {
		return fmt.Errorf("insert green_payment: %w", err)
	}

	return nil
}

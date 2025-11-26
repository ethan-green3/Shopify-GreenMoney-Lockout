package internal

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// Status constants for green_payments.current_status.
const (
	StatusPendingInvoice = "pending_invoice"
	StatusInvoiceSent    = "invoice_sent"
	StatusInvoiceError   = "invoice_error"
	StatusCleared        = "cleared"
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

// InsertPendingPayment inserts a new row into green_payments with status "pending_invoice"
// and returns the new row's id.
func InsertPendingPayment(db *sql.DB, order ShopifyOrder) (int64, error) {
	amount, err := strconv.ParseFloat(order.TotalPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid total_price %q: %w", order.TotalPrice, err)
	}

	query := `
		INSERT INTO green_payments (
			shopify_order_id,
			shopify_order_name,
			amount,
			currency,
			current_status,
			is_cleared,
			created_at,
			updated_at,
			last_status_at
		) VALUES ($1, $2, $3, $4, $5, FALSE, NOW(), NOW(), NOW())
		RETURNING id
	`

	var id int64
	err = db.QueryRow(
		query,
		order.ID,
		order.Name,
		amount,
		order.Currency,
		StatusPendingInvoice,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert pending green_payment: %w", err)
	}

	return id, nil
}

// UpdatePaymentAfterInvoice updates the row after calling OneTimeInvoice.
func UpdatePaymentAfterInvoice(db *sql.DB, id int64, invoiceID, checkID, status string) error {
	query := `
		UPDATE green_payments
		SET
			invoice_id = $1,
			green_check_id = $2,
			current_status = $3,
			updated_at = NOW(),
			last_status_at = NOW()
		WHERE id = $4
	`

	_, err := db.Exec(query, invoiceID, checkID, status, id)
	if err != nil {
		return fmt.Errorf("update after invoice: %w", err)
	}
	return nil
}

// GetPaymentByCheckID fetches a GreenPayment row by green_check_id.
func GetPaymentByCheckID(db *sql.DB, chkID string) (*GreenPayment, error) {
	query := `
		SELECT
			id,
			shopify_order_id,
			shopify_order_name,
			amount,
			currency,
			invoice_id,
			green_check_id,
			current_status,
			is_cleared,
			shopify_marked_paid_at,
			created_at,
			updated_at,
			last_status_at
		FROM green_payments
		WHERE green_check_id = $1
	`

	row := db.QueryRow(query, chkID)

	var gp GreenPayment
	err := row.Scan(
		&gp.ID,
		&gp.ShopifyOrderID,
		&gp.ShopifyOrderName,
		&gp.Amount,
		&gp.Currency,
		&gp.InvoiceID,
		&gp.GreenCheckID,
		&gp.CurrentStatus,
		&gp.IsCleared,
		&gp.ShopifyMarkedPaidAt,
		&gp.CreatedAt,
		&gp.UpdatedAt,
		&gp.LastStatusAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no payment found for chkID=%s", chkID)
		}
		return nil, fmt.Errorf("query payment: %w", err)
	}

	return &gp, nil
}

// MarkPaymentCleared updates the green_payments row to 'cleared' status
// based on the green_check_id from Green (ChkID).
func MarkPaymentCleared(db *sql.DB, chkID string) error {
	query := `
		UPDATE green_payments
		SET 
			current_status = $1,
			is_cleared = TRUE,
			shopify_marked_paid_at = NOW(),
			updated_at = NOW(),
			last_status_at = NOW()
		WHERE green_check_id = $2
	`

	res, err := db.Exec(query, StatusCleared, chkID)
	if err != nil {
		return fmt.Errorf("update green_payment: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no payment found with green_check_id=%s", chkID)
	}

	return nil
}

// ListPendingInvoicePayments returns all rows that are in 'invoice_sent' status
// and not yet cleared. These are the ones the poller should check with Green.
func ListPendingInvoicePayments(db *sql.DB) ([]GreenPayment, error) {
	query := `
		SELECT
			id,
			shopify_order_id,
			shopify_order_name,
			amount,
			currency,
			invoice_id,
			green_check_id,
			current_status,
			is_cleared,
			shopify_marked_paid_at,
			created_at,
			updated_at,
			last_status_at
		FROM green_payments
		WHERE current_status = $1
		  AND is_cleared = FALSE
	`

	rows, err := db.Query(query, StatusInvoiceSent)
	if err != nil {
		return nil, fmt.Errorf("query pending invoices: %w", err)
	}
	defer rows.Close()

	var result []GreenPayment

	for rows.Next() {
		var gp GreenPayment
		if err := rows.Scan(
			&gp.ID,
			&gp.ShopifyOrderID,
			&gp.ShopifyOrderName,
			&gp.Amount,
			&gp.Currency,
			&gp.InvoiceID,
			&gp.GreenCheckID,
			&gp.CurrentStatus,
			&gp.IsCleared,
			&gp.ShopifyMarkedPaidAt,
			&gp.CreatedAt,
			&gp.UpdatedAt,
			&gp.LastStatusAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending invoice: %w", err)
		}
		result = append(result, gp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

// SetCheckIDForInvoice updates green_check_id for a given Invoice_ID.
// We also bump updated_at and last_status_at so you can see when it changed.
func SetCheckIDForInvoice(db *sql.DB, invoiceID, checkID string) error {
	query := `
		UPDATE green_payments
		SET green_check_id = $1,
		    updated_at = NOW(),
		    last_status_at = NOW()
		WHERE invoice_id = $2
	`
	res, err := db.Exec(query, checkID, invoiceID)
	if err != nil {
		return fmt.Errorf("update green_check_id for invoice %s: %w", invoiceID, err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("no green_payments row found for invoice_id=%s", invoiceID)
	}
	return err
}

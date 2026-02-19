package moneyeu

import (
	"database/sql"
	"fmt"
)

// Store webhook payload and status
func StoreMoneyEUWebhookEvent(db *sql.DB, shopifyOrderID string, status string, rawPayload []byte) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET
			status = COALESCE(NULLIF($1,''), status),
			last_event_at = NOW(),
			last_webhook_payload = $2::jsonb,
			updated_at = NOW()
		WHERE shopify_order_id = $3
	`, status, string(rawPayload), shopifyOrderID)
	return err
}

// Idempotency check
func IsMoneyEUShopifyMarkedPaid(db *sql.DB, shopifyOrderID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT (shopify_marked_paid_at IS NOT NULL)
		FROM money_eu_payments
		WHERE shopify_order_id = $1
	`, shopifyOrderID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Pull amount/currency + Shopify numeric order id (stored as string in your current MoneyEU code)
func GetMoneyEUShopifyPaymentInfo(db *sql.DB, shopifyOrderID string) (amountStr string, currency string, shopifyNumericID int64, err error) {
	var amount float64
	var currencyDB string
	var orderIDStr string

	err = db.QueryRow(`
		SELECT amount, currency, shopify_order_id
		FROM money_eu_payments
		WHERE shopify_order_id = $1
	`, shopifyOrderID).Scan(&amount, &currencyDB, &orderIDStr)
	if err != nil {
		return "", "", 0, err
	}

	var id int64
	_, scanErr := fmt.Sscan(orderIDStr, &id)
	if scanErr != nil {
		return "", "", 0, fmt.Errorf("parse shopify_order_id=%q to int64: %w", orderIDStr, scanErr)
	}

	return fmt.Sprintf("%.2f", amount), currencyDB, id, nil
}

// Mark locally as paid (only once)
func MarkMoneyEUShopifyPaid(db *sql.DB, shopifyOrderID string) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET
			shopify_marked_paid_at = NOW(),
			paid_at = COALESCE(paid_at, NOW()),
			status = 'paid',
			updated_at = NOW()
		WHERE shopify_order_id = $1
		  AND shopify_marked_paid_at IS NULL
	`, shopifyOrderID)
	return err
}

func MarkMoneyEUFailed(db *sql.DB, shopifyOrderID string, reason string) error {
	_, err := db.Exec(`
        UPDATE money_eu_payments
        SET status='failed',
            failed_at=NOW(),
            failure_reason=LEFT($2, 500),
            updated_at=NOW()
        WHERE shopify_order_id=$1
    `, shopifyOrderID, reason)
	return err
}

func HasCheckoutLinkForOrder(db *sql.DB, shopifyOrderID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM money_eu_payments
			WHERE shopify_order_id = $1
			  AND COALESCE(NULLIF(checkout_url, ''), '') <> ''
		)
	`, shopifyOrderID).Scan(&exists)
	return exists, err
}

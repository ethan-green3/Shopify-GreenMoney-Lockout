package moneyeu

import (
	"database/sql"
	"fmt"
)

type ShopifyPaymentInfo struct {
	ShopDomain       string
	AmountStr        string
	Currency         string
	ShopifyNumericID int64
}

func GetMoneyEUPaymentInfoByOrderID(db *sql.DB, shopifyOrderID string) (*ShopifyPaymentInfo, error) {
	rows, err := db.Query(`
		SELECT shop_domain, amount, currency, shopify_order_id
		FROM money_eu_payments
		WHERE shopify_order_id = $1
		ORDER BY updated_at DESC, created_at DESC, id DESC
		LIMIT 2
	`, shopifyOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*ShopifyPaymentInfo
	for rows.Next() {
		var amount float64
		var currencyDB string
		var orderIDStr string
		var domain string

		if err := rows.Scan(&domain, &amount, &currencyDB, &orderIDStr); err != nil {
			return nil, err
		}

		var id int64
		_, scanErr := fmt.Sscan(orderIDStr, &id)
		if scanErr != nil {
			return nil, fmt.Errorf("parse shopify_order_id=%q to int64: %w", orderIDStr, scanErr)
		}

		matches = append(matches, &ShopifyPaymentInfo{
			ShopDomain:       domain,
			AmountStr:        fmt.Sprintf("%.2f", amount),
			Currency:         currencyDB,
			ShopifyNumericID: id,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple money_eu_payments rows found for shopify_order_id=%s; webhook is ambiguous without shop_domain", shopifyOrderID)
	}

	return matches[0], nil
}

// Store webhook payload and status
func StoreMoneyEUWebhookEvent(db *sql.DB, shopDomain string, shopifyOrderID string, status string, rawPayload []byte) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET
			status = COALESCE(NULLIF($1,''), status),
			last_event_at = NOW(),
			last_webhook_payload = $2::jsonb,
			updated_at = NOW()
		WHERE shop_domain = $3
		  AND shopify_order_id = $4
	`, status, string(rawPayload), shopDomain, shopifyOrderID)
	return err
}

// Idempotency check
func IsMoneyEUShopifyMarkedPaid(db *sql.DB, shopDomain string, shopifyOrderID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT (shopify_marked_paid_at IS NOT NULL)
		FROM money_eu_payments
		WHERE shop_domain = $1
		  AND shopify_order_id = $2
	`, shopDomain, shopifyOrderID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Pull amount/currency + Shopify numeric order id (stored as string in your current MoneyEU code)
func GetMoneyEUShopifyPaymentInfo(db *sql.DB, shopDomain string, shopifyOrderID string) (*ShopifyPaymentInfo, error) {
	var amount float64
	var currencyDB string
	var orderIDStr string
	var domain string

	err := db.QueryRow(`
		SELECT shop_domain, amount, currency, shopify_order_id
		FROM money_eu_payments
		WHERE shop_domain = $1
		  AND shopify_order_id = $2
	`, shopDomain, shopifyOrderID).Scan(&domain, &amount, &currencyDB, &orderIDStr)
	if err != nil {
		return nil, err
	}

	var id int64
	_, scanErr := fmt.Sscan(orderIDStr, &id)
	if scanErr != nil {
		return nil, fmt.Errorf("parse shopify_order_id=%q to int64: %w", orderIDStr, scanErr)
	}

	return &ShopifyPaymentInfo{
		ShopDomain:       domain,
		AmountStr:        fmt.Sprintf("%.2f", amount),
		Currency:         currencyDB,
		ShopifyNumericID: id,
	}, nil
}

// Mark locally as paid (only once)
func MarkMoneyEUShopifyPaid(db *sql.DB, shopDomain string, shopifyOrderID string) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET
			shopify_marked_paid_at = NOW(),
			paid_at = COALESCE(paid_at, NOW()),
			status = 'paid',
			updated_at = NOW()
		WHERE shop_domain = $1
		  AND shopify_order_id = $2
		  AND shopify_marked_paid_at IS NULL
	`, shopDomain, shopifyOrderID)
	return err
}

func MarkMoneyEUFailed(db *sql.DB, shopDomain string, shopifyOrderID string, reason string) error {
	_, err := db.Exec(`
        UPDATE money_eu_payments
        SET status='failed',
            failed_at=NOW(),
            failure_reason=LEFT($3, 500),
            updated_at=NOW()
        WHERE shop_domain=$1
          AND shopify_order_id=$2
    `, shopDomain, shopifyOrderID, reason)
	return err
}

func HasCheckoutLinkForOrder(db *sql.DB, shopDomain string, shopifyOrderID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM money_eu_payments
			WHERE shop_domain = $1
			  AND shopify_order_id = $2
			  AND COALESCE(NULLIF(checkout_url, ''), '') <> ''
		)
	`, shopDomain, shopifyOrderID).Scan(&exists)
	return exists, err
}

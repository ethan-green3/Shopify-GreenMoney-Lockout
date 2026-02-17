package moneyeu

import (
	"database/sql"
	"fmt"
)

type PaymentRow struct {
	ShopifyOrderID   string
	ShopifyOrderName string
	Amount           float64
	Currency         string
	CustomerEmail    string
	CustomerName     string
	CustomerPhone    string
}

func InsertMoneyEUPayment(db *sql.DB, r PaymentRow) (int64, error) {
	var id int64
	err := db.QueryRow(`
		INSERT INTO money_eu_payments
			(shopify_order_id, shopify_order_name, amount, currency, customer_email, customer_name, customer_phone, current_status)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,'created')
		RETURNING id
	`, r.ShopifyOrderID, r.ShopifyOrderName, r.Amount, r.Currency, r.CustomerEmail, r.CustomerName, r.CustomerPhone).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert money_eu_payments: %w", err)
	}
	return id, nil
}

func SetMoneyEUOrderLink(db *sql.DB, paymentID int64, moneyEUOrderID, idOrderExt, url, status string) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET money_eu_order_id=$1,
			id_order_ext=$2,
			checkout_url=$3,
			money_eu_status=$4,
			current_status='link_created',
			updated_at=NOW(),
			last_status_at=NOW()
		WHERE id=$5
	`, moneyEUOrderID, idOrderExt, url, status, paymentID)
	return err
}

func MarkEmailSent(db *sql.DB, paymentID int64) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET email_sent_at=NOW(),
			email_status='sent',
			current_status='email_sent',
			updated_at=NOW(),
			last_status_at=NOW()
		WHERE id=$1
	`, paymentID)
	return err
}

func MarkEmailFailed(db *sql.DB, paymentID int64, errMsg string) error {
	_, err := db.Exec(`
		UPDATE money_eu_payments
		SET email_status='failed',
			email_error=$2,
			updated_at=NOW(),
			last_status_at=NOW()
		WHERE id=$1
	`, paymentID, errMsg)
	return err
}

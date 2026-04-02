package internal

import (
	"database/sql/driver"
	"testing"
	"time"

	"Shopify-GreenMoney-Lockout/internal/testsql"
)

func TestInsertPendingPaymentIncludesShopDomain(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "INSERT INTO green_payments",
			Args:          []any{"shop-a.myshopify.com", int64(2001), "#2001", 12.34, "USD", StatusPendingInvoice},
			Columns:       []string{"id"},
			Rows:          [][]driver.Value{{int64(99)}},
		},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	id, err := InsertPendingPayment(db, ShopifyOrder{
		ID:         2001,
		Name:       "#2001",
		ShopDomain: "shop-a.myshopify.com",
		TotalPrice: "12.34",
		Currency:   "USD",
	})
	if err != nil {
		t.Fatalf("InsertPendingPayment returned error: %v", err)
	}
	if id != 99 {
		t.Fatalf("unexpected id: %d", id)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestListPendingInvoicePaymentsScansShopDomain(t *testing.T) {
	now := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "FROM green_payments",
			Columns: []string{
				"id", "shop_domain", "shopify_order_id", "shopify_order_name", "invoice_id", "green_check_id",
				"amount", "currency", "current_status", "is_cleared", "last_status_at", "shopify_marked_paid_at", "processed_at",
			},
			Rows: [][]driver.Value{{
				int64(1), "shop-b.myshopify.com", int64(3001), "#3001", "inv-3001", "chk-3001",
				float64(88.5), "USD", StatusInvoiceSent, false, now, nil, nil,
			}},
		},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	rows, err := ListPendingInvoicePayments(db)
	if err != nil {
		t.Fatalf("ListPendingInvoicePayments returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].ShopDomain != "shop-b.myshopify.com" || rows[0].ShopifyOrderID != 3001 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

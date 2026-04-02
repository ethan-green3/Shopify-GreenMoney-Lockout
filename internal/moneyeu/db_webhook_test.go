package moneyeu

import (
	"database/sql/driver"
	"testing"

	"Shopify-GreenMoney-Lockout/internal/testsql"
)

func TestGetMoneyEUShopifyPaymentInfoUsesScopedLookup(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id",
			Args:          []any{"shop-2.myshopify.com", "12345"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows:          [][]driver.Value{{"shop-2.myshopify.com", float64(19.95), "USD", "12345"}},
		},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	info, err := GetMoneyEUShopifyPaymentInfo(db, "shop-2.myshopify.com", "12345")
	if err != nil {
		t.Fatalf("GetMoneyEUShopifyPaymentInfo returned error: %v", err)
	}
	if info.ShopDomain != "shop-2.myshopify.com" || info.AmountStr != "19.95" || info.ShopifyNumericID != 12345 {
		t.Fatalf("unexpected payment info: %+v", info)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestHasCheckoutLinkForOrderUsesScopedLookup(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT EXISTS",
			Args:          []any{"shop-2.myshopify.com", "12345"},
			Columns:       []string{"exists"},
			Rows:          [][]driver.Value{{true}},
		},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	exists, err := HasCheckoutLinkForOrder(db, "shop-2.myshopify.com", "12345")
	if err != nil {
		t.Fatalf("HasCheckoutLinkForOrder returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected checkout link to exist")
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

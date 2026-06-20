package moneyeu

import (
	"bytes"
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"

	"Shopify-GreenMoney-Lockout/internal/testsql"
)

type fakeResolver struct {
	t          *testing.T
	wantDomain string
	payer      ShopifyPayer
}

func (r fakeResolver) ForShopDomain(shopDomain string) (ShopifyPayer, error) {
	if shopDomain != r.wantDomain {
		r.t.Fatalf("unexpected shop domain lookup: %q", shopDomain)
	}
	return r.payer, nil
}

type fakePayer struct {
	called   bool
	orderID  int64
	amount   string
	currency string
}

func (p *fakePayer) MarkOrderPaid(_ context.Context, orderID int64, amount string, currency string) error {
	p.called = true
	p.orderID = orderID
	p.amount = amount
	p.currency = currency
	return nil
}

func TestMoneyEUWebhookHandlerPaidFlowUsesShopScopedLookup(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id FROM money_eu_payments WHERE shopify_order_id = $1",
			Args:          []any{"123"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows:          [][]driver.Value{{"secondary.myshopify.com", float64(49.99), "USD", "123"}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"success", `{"transaction_id":247201,"orderidext":"123","response_message":"Payment successful","paid_amount":49.99,"currency":"USD","transaction_id_ref":"abc","status":"Success"}`, "secondary.myshopify.com", "123"},
			RowsAffected:  1,
		},
		{
			Kind:          "query",
			QueryContains: "SELECT (shopify_marked_paid_at IS NOT NULL) FROM money_eu_payments",
			Args:          []any{"secondary.myshopify.com", "123"},
			Columns:       []string{"shopify_marked_paid_at"},
			Rows:          [][]driver.Value{{false}},
		},
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id FROM money_eu_payments",
			Args:          []any{"secondary.myshopify.com", "123"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows:          [][]driver.Value{{"secondary.myshopify.com", float64(49.99), "USD", "123"}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"secondary.myshopify.com", "123"},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	payer := &fakePayer{}
	resolver := fakeResolver{
		t:          t,
		wantDomain: "secondary.myshopify.com",
		payer:      payer,
	}

	body := []byte(`{"transaction_id":247201,"orderidext":"123","response_message":"Payment successful","paid_amount":49.99,"currency":"USD","transaction_id_ref":"abc","status":"Success"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	MoneyEUWebhookHandler(db, resolver).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if !payer.called || payer.orderID != 123 || payer.amount != "49.99" || payer.currency != "USD" {
		t.Fatalf("unexpected payer call: %+v", payer)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestMoneyEUWebhookHandlerFailedFlowMarksFailureForScopedOrder(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id FROM money_eu_payments WHERE shopify_order_id = $1",
			Args:          []any{"123"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows:          [][]driver.Value{{"secondary.myshopify.com", float64(49.99), "USD", "123"}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"declined", `{"transaction_id":247202,"orderidext":"123","response_message":"30: Invalid Card","paid_amount":2,"currency":"USD","transaction_id_ref":"0","status":"Declined"}`, "secondary.myshopify.com", "123"},
			RowsAffected:  1,
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET status='failed'",
			Args:          []any{"secondary.myshopify.com", "123", "30: Invalid Card"},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	body := []byte(`{"transaction_id":247202,"orderidext":"123","response_message":"30: Invalid Card","paid_amount":2,"currency":"USD","transaction_id_ref":"0","status":"Declined"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	MoneyEUWebhookHandler(db, fakeResolver{t: t, wantDomain: "unused", payer: &fakePayer{}}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestMoneyEUWebhookHandlerProcessFlowStoresEventOnly(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id FROM money_eu_payments WHERE shopify_order_id = $1",
			Args:          []any{"123"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows:          [][]driver.Value{{"secondary.myshopify.com", float64(49.99), "USD", "123"}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"process", `{"transaction_id":247203,"orderidext":"123","response_message":"Processing","paid_amount":0,"currency":"USD","transaction_id_ref":"0","status":"Process"}`, "secondary.myshopify.com", "123"},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	payer := &fakePayer{}
	body := []byte(`{"transaction_id":247203,"orderidext":"123","response_message":"Processing","paid_amount":0,"currency":"USD","transaction_id_ref":"0","status":"Process"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	MoneyEUWebhookHandler(db, fakeResolver{t: t, wantDomain: "unused", payer: payer}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if payer.called {
		t.Fatal("did not expect Shopify mark-paid call for Process status")
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestGetMoneyEUPaymentInfoByOrderIDAmbiguous(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT shop_domain, amount, currency, shopify_order_id FROM money_eu_payments WHERE shopify_order_id = $1",
			Args:          []any{"123"},
			Columns:       []string{"shop_domain", "amount", "currency", "shopify_order_id"},
			Rows: [][]driver.Value{
				{"shop-a.myshopify.com", float64(10), "USD", "123"},
				{"shop-b.myshopify.com", float64(10), "USD", "123"},
			},
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	_, err = GetMoneyEUPaymentInfoByOrderID(db, "123")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

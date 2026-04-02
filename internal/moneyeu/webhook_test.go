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

func TestMoneyEUWebhookHandlerRequiresShopDomain(t *testing.T) {
	body := []byte(`{"content":{"idOrderExt":"123","status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	MoneyEUWebhookHandler(nil, fakeResolver{t: t, wantDomain: "unused", payer: &fakePayer{}}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
}

func TestMoneyEUWebhookHandlerPaidFlowUsesShopScopedLookup(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"paid", `{"content":{"idOrderExt":"123","status":"paid"}}`, "secondary.myshopify.com", "123"},
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

	body := []byte(`{"content":{"idOrderExt":"123","status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu?shop_domain=secondary.myshopify.com", bytes.NewReader(body))
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
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET",
			Args:          []any{"failed", `{"message":"declined","content":{"idOrderExt":"123","status":"failed"}}`, "secondary.myshopify.com", "123"},
			RowsAffected:  1,
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments SET status='failed'",
			Args:          []any{"secondary.myshopify.com", "123", "declined"},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	body := []byte(`{"message":"declined","content":{"idOrderExt":"123","status":"failed"}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/moneyeu?shop_domain=secondary.myshopify.com", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	MoneyEUWebhookHandler(db, fakeResolver{t: t, wantDomain: "unused", payer: &fakePayer{}}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "okok" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

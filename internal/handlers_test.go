package internal

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"Shopify-GreenMoney-Lockout/internal/moneyeu"
	"Shopify-GreenMoney-Lockout/internal/testsql"
)

func TestShopifyOrderCreateHandlerRequiresShopDomainHeader(t *testing.T) {
	body, err := json.Marshal(ShopifyOrder{
		ID:                  123,
		Name:                "#1001",
		PaymentGatewayNames: []string{"Credit/Debit Card"},
	})
	if err != nil {
		t.Fatalf("marshal order: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/shopify/orders-create", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	ShopifyOrderCreateHandler(nil, nil, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestShopifyOrderCreateHandlerRoutesGreenPath(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "INSERT INTO green_payments",
			Args:          []any{"green-store.myshopify.com", int64(123), "#1001", 42.5, "USD", StatusPendingInvoice},
			Columns:       []string{"id"},
			Rows:          [][]driver.Value{{int64(55)}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE green_payments",
			Args:          []any{"inv-1", "chk-1", StatusInvoiceSent, int64(55)},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/echeck.asmx/OneTimeInvoice" {
			t.Fatalf("unexpected Green path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("CustomerName") != "Jane Doe" {
			t.Fatalf("unexpected CustomerName: %q", values.Get("CustomerName"))
		}
		if values.Get("EmailAddress") != "jane@example.com" {
			t.Fatalf("unexpected EmailAddress: %q", values.Get("EmailAddress"))
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<InvoiceResult xmlns="CheckProcessing"><PaymentResult>0</PaymentResult><PaymentResultDescription>ok</PaymentResultDescription><Invoice_ID>inv-1</Invoice_ID><Check_ID>chk-1</Check_ID></InvoiceResult>`))
	}))
	defer server.Close()

	green := &GreenClient{
		BaseURL:     server.URL,
		ClientID:    "client",
		APIPassword: "secret",
		HTTPClient:  server.Client(),
	}

	body, _ := json.Marshal(ShopifyOrder{
		ID:                  123,
		Name:                "#1001",
		Email:               "jane@example.com",
		TotalPrice:          "42.50",
		Currency:            "USD",
		PaymentGatewayNames: []string{"Green Money"},
		BillingAddress:      &ShopifyAddress{FirstName: "Jane", LastName: "Doe"},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/shopify/orders-create", bytes.NewReader(body))
	req.Header.Set(ShopifyShopDomainHeader, "green-store.myshopify.com")
	rr := httptest.NewRecorder()

	ShopifyOrderCreateHandler(db, green, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "invoice_sent" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestShopifyOrderCreateHandlerRoutesMoneyEUPath(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "SELECT EXISTS ( SELECT 1 FROM money_eu_payments",
			Args:          []any{"secondary.myshopify.com", "123"},
			Columns:       []string{"exists"},
			Rows:          [][]driver.Value{{false}},
		},
		{
			Kind:          "query",
			QueryContains: "INSERT INTO money_eu_payments",
			Args:          []any{"secondary.myshopify.com", "123", "#1001", 49.99, "USD", "buyer@example.com", "John Buyer", "5551234567"},
			Columns:       []string{"id"},
			Rows:          [][]driver.Value{{int64(77)}},
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE money_eu_payments",
			Args:          []any{"987", "123", "https://checkout.test/123", "paid", int64(77)},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	var gotReq moneyeu.CreateOrderExtRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/createOrderExt" {
			t.Fatalf("unexpected MoneyEU path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode MoneyEU request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","content":{"id":987,"idOrderExt":"123","url":"https://checkout.test/123","status":"paid"}}`))
	}))
	defer server.Close()

	moneySvc := &moneyeu.Service{
		DB: db,
		Client: &moneyeu.Client{
			BaseURL:   server.URL,
			APIKey:    "key",
			APISecret: "secret",
			HTTP:      server.Client(),
		},
	}

	body, _ := json.Marshal(map[string]any{
		"id":                    123,
		"name":                  "#1001",
		"email":                 "buyer@example.com",
		"total_price":           "49.99",
		"currency":              "USD",
		"payment_gateway_names": []string{"Credit/Debit Card"},
		"shipping_address": map[string]any{
			"first_name":   "John",
			"last_name":    "Buyer",
			"phone":        "5551234567",
			"address1":     "123 Main St",
			"city":         "Austin",
			"province":     "TX",
			"zip":          "78701",
			"country_code": "US",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/shopify/orders-create", bytes.NewReader(body))
	req.Header.Set(ShopifyShopDomainHeader, "secondary.myshopify.com")
	rr := httptest.NewRecorder()

	ShopifyOrderCreateHandler(db, nil, moneySvc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.String() != "moneyeu_email_sent" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Body.String())
	}
	if gotReq.IdOrderExt != "123" || gotReq.Name != "John Buyer" || gotReq.Mail != "buyer@example.com" {
		t.Fatalf("unexpected MoneyEU request core fields: %+v", gotReq)
	}
	if gotReq.PhoneNumber != "5551234567" || gotReq.Address != "123 Main St" || gotReq.City != "Austin" || gotReq.State != "TX" || gotReq.Zip != "78701" {
		t.Fatalf("unexpected MoneyEU address fields: %+v", gotReq)
	}
	if gotReq.Country != "United States" || gotReq.DialCode != "+1" {
		t.Fatalf("unexpected MoneyEU country fields: %+v", gotReq)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestGreenIPNHandlerMarksPaidWithShopSpecificClient(t *testing.T) {
	db, state, err := testsql.Open([]testsql.Expectation{
		{
			Kind:          "query",
			QueryContains: "FROM green_payments WHERE green_check_id = $1",
			Args:          []any{"chk-1"},
			Columns: []string{
				"id", "shop_domain", "shopify_order_id", "shopify_order_name", "amount", "currency",
				"invoice_id", "green_check_id", "current_status", "is_cleared", "shopify_marked_paid_at",
				"created_at", "updated_at", "last_status_at", "processed_at",
			},
			Rows: func() [][]driver.Value {
				now := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
				return [][]driver.Value{{
					int64(1), "secondary.myshopify.com", int64(9001), "#1001", float64(42.5), "USD",
					"inv-1", "chk-1", "invoice_sent", false, nil, now, now, now, nil,
				}}
			}(),
		},
		{
			Kind:          "exec",
			QueryContains: "UPDATE green_payments SET",
			Args:          []any{StatusCleared, "chk-1"},
			RowsAffected:  1,
		},
	})
	if err != nil {
		t.Fatalf("open testsql db: %v", err)
	}
	defer db.Close()

	var gotPath string
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Shopify-Access-Token")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	client := NewShopifyClient("secondary.myshopify.com", "store-2-token", "2024-10")
	client.HTTPClient = &http.Client{
		Transport: rewriteTransport{
			target: targetURL,
			base:   http.DefaultTransport,
		},
	}

	registry := NewShopifyClientRegistry("2024-10")
	registry.Register(client)

	req := httptest.NewRequest(http.MethodGet, "/green/ipn?ChkID=chk-1&TransID=trans-1", nil)
	rr := httptest.NewRecorder()

	GreenIPNHandler(db, registry, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%q", rr.Code, rr.Body.String())
	}
	if gotPath != "/admin/api/2024-10/orders/9001/transactions.json" {
		t.Fatalf("unexpected Shopify path: %q", gotPath)
	}
	if gotToken != "store-2-token" {
		t.Fatalf("unexpected Shopify token: %q", gotToken)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	if clone.Body == nil {
		clone.Body = io.NopCloser(bytes.NewReader(nil))
	}
	return t.base.RoundTrip(clone)
}

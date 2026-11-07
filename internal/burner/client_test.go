package burner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateInvoiceUsesBearerVersionAndIdempotency(t *testing.T) {
	t.Setenv("BURNER_ALLOW_INSECURE_HTTP", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/invoices" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing auth")
		}
		if r.Header.Get("Burner-Version") != "2026-01-01" {
			t.Fatalf("missing version")
		}
		if r.Header.Get("Idempotency-Key") != "order-key" {
			t.Fatalf("missing idempotency key")
		}
		var body InvoiceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.AmountUSD != "12.34" {
			t.Fatalf("amount=%q", body.AmountUSD)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inv-1","status":"pending","amount":"12.34"}`))
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, APIKey: "secret", APIVersion: "2026-01-01", HTTP: server.Client()}
	inv, err := c.CreateInvoice(context.Background(), "order-key", InvoiceCreateRequest{AmountUSD: "12.34", Metadata: map[string]any{"source": "shopify"}})
	if err != nil {
		t.Fatal(err)
	}
	if inv.ID != "inv-1" {
		t.Fatalf("id=%s", inv.ID)
	}
}

func TestCheckoutURLEscapesFields(t *testing.T) {
	u, err := CheckoutURL("https://pay.burner.pro", "inv/1", CheckoutFields{Email: "a+b@example.com", FirstName: "A & B", ReturnURL: "https://shop.test/orders/1?a=1&b=2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "email=a%2Bb%40example.com") || !strings.Contains(u, "return_url=https%3A%2F%2Fshop.test%2Forders%2F1%3Fa%3D1%26b%3D2") {
		t.Fatalf("URL not encoded: %s", u)
	}
}

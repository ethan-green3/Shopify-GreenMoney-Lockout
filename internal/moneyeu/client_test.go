package moneyeu

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCheckoutOrderUsesFullProcessPaymentURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"Order created successfully","data":{"orderId":"ORD-1","checkoutUrl":"https://checkout.test/1"}}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:           "https://wrong.example",
		ProcessPaymentURL: server.URL + "/moneyEu/api/v1/processPayment",
		APIKey:            "key",
		APISecret:         "secret",
		HTTP:              server.Client(),
		Path:              "/wrong",
	}

	_, err := client.CreateCheckoutOrder(context.Background(), ProcessPaymentRequest{
		CustomerName:  "John Buyer",
		CustomerEmail: "buyer@example.com",
		Amount:        5.99,
		Currency:      "USD",
	})
	if err != nil {
		t.Fatalf("CreateCheckoutOrder returned error: %v", err)
	}
	if gotPath != "/moneyEu/api/v1/processPayment" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}

func TestCreateCheckoutOrderSignsRawBody(t *testing.T) {
	var body string
	var signature string
	var salt string
	var timestamp string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		body = string(raw)
		signature = r.Header.Get("signature")
		salt = r.Header.Get("salt")
		timestamp = r.Header.Get("timestamp")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"Order created successfully","data":{"orderId":"ORD-1","checkoutUrl":"https://checkout.test/1"}}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:   server.URL,
		APIKey:    "key",
		APISecret: "secret",
		HTTP:      server.Client(),
		Path:      "/moneyEu/api/v1/processPayment",
	}

	_, err := client.CreateCheckoutOrder(context.Background(), ProcessPaymentRequest{
		CustomerName:  "John Buyer",
		CustomerEmail: "buyer@example.com",
		Amount:        5.99,
		Currency:      "USD",
	})
	if err != nil {
		t.Fatalf("CreateCheckoutOrder returned error: %v", err)
	}

	want := buildSignature("secret", salt, "key", timestamp, body)
	if signature != want {
		t.Fatalf("unexpected signature: got %q want %q", signature, want)
	}
}

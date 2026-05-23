package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMarkOrderPaidUsesClientIDBasicAuthForLockoutSupplements2(t *testing.T) {
	var gotUser string
	var gotPassword string
	var gotAccessToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPassword, _ = r.BasicAuth()
		gotAccessToken = r.Header.Get("X-Shopify-Access-Token")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	client := NewShopifyClient("lockoutsupplements2.myshopify.com", "app-secret", "2024-10")
	client.ClientID = "client-id-2"
	client.HTTPClient = &http.Client{
		Transport: rewriteTransport{
			target: targetURL,
			base:   http.DefaultTransport,
		},
	}

	if err := client.MarkOrderPaid(context.Background(), 123, "49.99", "USD"); err != nil {
		t.Fatalf("MarkOrderPaid returned error: %v", err)
	}

	if gotUser != "client-id-2" || gotPassword != "app-secret" {
		t.Fatalf("unexpected basic auth: user=%q password=%q", gotUser, gotPassword)
	}
	if gotAccessToken != "" {
		t.Fatalf("expected no access token header, got %q", gotAccessToken)
	}
}

func TestMarkOrderPaidRequiresClientIDForLockoutSupplements2(t *testing.T) {
	client := NewShopifyClient("lockoutsupplements2.myshopify.com", "app-secret", "2024-10")

	err := client.MarkOrderPaid(context.Background(), 123, "49.99", "USD")
	if err == nil {
		t.Fatal("expected missing client ID error")
	}
}

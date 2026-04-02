package internal

import (
	"net/http/httptest"
	"testing"
)

func TestExtractShopDomain(t *testing.T) {
	req := httptest.NewRequest("POST", "/webhooks/shopify/orders-create", nil)
	req.Header.Set(ShopifyShopDomainHeader, "  Store-2.MyShopify.Com ")

	got, err := ExtractShopDomain(req)
	if err != nil {
		t.Fatalf("ExtractShopDomain returned error: %v", err)
	}
	if got != "store-2.myshopify.com" {
		t.Fatalf("unexpected shop domain: %q", got)
	}
}

func TestExtractShopDomainMissingHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/webhooks/shopify/orders-create", nil)

	if _, err := ExtractShopDomain(req); err == nil {
		t.Fatal("expected error for missing shop domain header")
	}
}

func TestPaymentGatewayRoutingHelpers(t *testing.T) {
	order := ShopifyOrder{PaymentGatewayNames: []string{"Something Else", "Green Money", "Credit/Debit Card"}}

	if !IsGreenMoneyOrder(order) {
		t.Fatal("expected Green Money order detection")
	}
	if !IsMoneyEUOrder(order) {
		t.Fatal("expected MoneyEU order detection")
	}
}

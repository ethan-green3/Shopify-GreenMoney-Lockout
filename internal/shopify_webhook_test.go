package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http/httptest"
	"testing"
)

func TestShopifyWebhookSecretsVerify(t *testing.T) {
	secret := NewShopifyWebhookSecrets()
	secret.Register("shop.example", "secret")
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("X-Shopify-Shop-Domain", "shop.example")
	body := []byte(`{"id":1}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	r.Header.Set("X-Shopify-Hmac-Sha256", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	if err := secret.Verify(r, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	r.Header.Set("X-Shopify-Hmac-Sha256", "not-base64")
	if err := secret.Verify(r, body); err == nil {
		t.Fatal("malformed signature accepted")
	}
}

package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// ShopifyWebhookSecrets stores the app secret for each configured shop.
// Shopify signs the exact raw request body with this secret.
type ShopifyWebhookSecrets struct {
	byDomain map[string]string
}

func NewShopifyWebhookSecrets() *ShopifyWebhookSecrets {
	return &ShopifyWebhookSecrets{byDomain: make(map[string]string)}
}

func (s *ShopifyWebhookSecrets) Register(domain, secret string) {
	if s == nil {
		return
	}
	domain = normalizeShopDomain(domain)
	secret = strings.TrimSpace(secret)
	if domain != "" && secret != "" {
		s.byDomain[domain] = secret
	}
}

func (s *ShopifyWebhookSecrets) HasAny() bool {
	return s != nil && len(s.byDomain) > 0
}

func (s *ShopifyWebhookSecrets) Verify(r *http.Request, raw []byte) error {
	if s == nil {
		return fmt.Errorf("Shopify webhook verifier is not configured")
	}
	domain := normalizeShopDomain(r.Header.Get(ShopifyShopDomainHeader))
	secret, ok := s.byDomain[domain]
	if !ok {
		return fmt.Errorf("no Shopify webhook secret configured for shop domain %s", domain)
	}
	provided := strings.TrimSpace(r.Header.Get("X-Shopify-Hmac-Sha256"))
	if provided == "" {
		return fmt.Errorf("missing Shopify webhook HMAC")
	}
	decoded, err := base64.StdEncoding.DecodeString(provided)
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("invalid Shopify webhook HMAC")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	if subtle.ConstantTimeCompare(decoded, mac.Sum(nil)) != 1 {
		return fmt.Errorf("invalid Shopify webhook HMAC")
	}
	return nil
}

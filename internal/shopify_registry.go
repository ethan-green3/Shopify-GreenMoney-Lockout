package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type ShopifyClientRegistry struct {
	clients           map[string]*ShopifyClient
	defaultAPIVersion string
}

type shopifyStoreEnvConfig struct {
	AccessToken string `json:"access_token"`
	APIVersion  string `json:"api_version"`
}

func NewShopifyClientRegistry(defaultAPIVersion string) *ShopifyClientRegistry {
	return &ShopifyClientRegistry{
		clients:           make(map[string]*ShopifyClient),
		defaultAPIVersion: strings.TrimSpace(defaultAPIVersion),
	}
}

func NewShopifyClientRegistryFromEnv() (*ShopifyClientRegistry, error) {
	defaultVersion := strings.TrimSpace(os.Getenv("SHOPIFY_API_VERSION"))
	registry := NewShopifyClientRegistry(defaultVersion)

	registerStoreFromEnv(registry, "SHOPIFY_STORE_DOMAIN", "SHOPIFY_ACCESS_TOKEN", defaultVersion)
	registerStoreFromEnv(registry, "SHOPIFY_STORE_DOMAIN2", "SHOPIFY_ACCESS_TOKEN2", defaultVersion)

	rawConfigs := strings.TrimSpace(os.Getenv("SHOPIFY_STORE_CONFIGS"))
	if rawConfigs == "" {
		return registry, nil
	}

	var configs map[string]shopifyStoreEnvConfig
	if err := json.Unmarshal([]byte(rawConfigs), &configs); err != nil {
		return nil, fmt.Errorf("parse SHOPIFY_STORE_CONFIGS: %w", err)
	}

	for domain, cfg := range configs {
		normalizedDomain := normalizeShopDomain(domain)
		if normalizedDomain == "" {
			continue
		}

		apiVersion := strings.TrimSpace(cfg.APIVersion)
		if apiVersion == "" {
			apiVersion = defaultVersion
		}

		if strings.TrimSpace(cfg.AccessToken) == "" {
			return nil, fmt.Errorf("missing access_token for Shopify store %s", normalizedDomain)
		}

		registry.Register(NewShopifyClient(normalizedDomain, cfg.AccessToken, apiVersion))
	}

	return registry, nil
}

func (r *ShopifyClientRegistry) Register(client *ShopifyClient) {
	if client == nil {
		return
	}

	domain := normalizeShopDomain(client.StoreDomain)
	if domain == "" {
		return
	}

	client.StoreDomain = domain
	if client.APIVersion == "" {
		client.APIVersion = r.defaultAPIVersion
	}
	if client.HTTPClient == nil {
		client.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	r.clients[domain] = client
}

func (r *ShopifyClientRegistry) ForShopDomain(shopDomain string) (*ShopifyClient, error) {
	domain := normalizeShopDomain(shopDomain)
	if domain == "" {
		return nil, fmt.Errorf("shop domain is required")
	}

	client, ok := r.clients[domain]
	if !ok || client == nil {
		return nil, fmt.Errorf("no Shopify client configured for shop domain %s", domain)
	}
	if client.AccessToken == "" {
		return nil, fmt.Errorf("Shopify client missing access token for shop domain %s", domain)
	}
	return client, nil
}

func (r *ShopifyClientRegistry) HasAny() bool {
	return len(r.clients) > 0
}

func normalizeShopDomain(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func registerStoreFromEnv(registry *ShopifyClientRegistry, domainEnv, tokenEnv, defaultVersion string) {
	domain := normalizeShopDomain(os.Getenv(domainEnv))
	token := strings.TrimSpace(os.Getenv(tokenEnv))
	if domain == "" || token == "" {
		return
	}

	registry.Register(NewShopifyClient(domain, token, defaultVersion))
}

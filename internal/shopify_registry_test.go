package internal

import "testing"

func TestNewShopifyClientRegistryFromEnvLoadsPrimaryAndSecondaryStores(t *testing.T) {
	t.Setenv("SHOPIFY_API_VERSION", "2024-10")
	t.Setenv("SHOPIFY_STORE_DOMAIN", "primary.myshopify.com")
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "token-1")
	t.Setenv("SHOPIFY_STORE_DOMAIN2", "secondary.myshopify.com")
	t.Setenv("SHOPIFY_ACCESS_TOKEN2", "token-2")
	t.Setenv("SHOPIFY_STORE_CONFIGS", "")

	registry, err := NewShopifyClientRegistryFromEnv()
	if err != nil {
		t.Fatalf("NewShopifyClientRegistryFromEnv returned error: %v", err)
	}

	primary, err := registry.ForShopDomain("primary.myshopify.com")
	if err != nil {
		t.Fatalf("lookup primary store: %v", err)
	}
	if primary.AccessToken != "token-1" || primary.APIVersion != "2024-10" {
		t.Fatalf("unexpected primary client: %+v", primary)
	}

	secondary, err := registry.ForShopDomain("secondary.myshopify.com")
	if err != nil {
		t.Fatalf("lookup secondary store: %v", err)
	}
	if secondary.AccessToken != "token-2" || secondary.APIVersion != "2024-10" {
		t.Fatalf("unexpected secondary client: %+v", secondary)
	}
}

func TestNewShopifyClientRegistryFromEnvLoadsJSONConfigs(t *testing.T) {
	t.Setenv("SHOPIFY_API_VERSION", "2024-10")
	t.Setenv("SHOPIFY_STORE_CONFIGS", `{"json-store.myshopify.com":{"access_token":"json-token","api_version":"2025-01"}}`)
	t.Setenv("SHOPIFY_STORE_DOMAIN", "")
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "")
	t.Setenv("SHOPIFY_STORE_DOMAIN2", "")
	t.Setenv("SHOPIFY_ACCESS_TOKEN2", "")

	registry, err := NewShopifyClientRegistryFromEnv()
	if err != nil {
		t.Fatalf("NewShopifyClientRegistryFromEnv returned error: %v", err)
	}

	client, err := registry.ForShopDomain("json-store.myshopify.com")
	if err != nil {
		t.Fatalf("lookup JSON store: %v", err)
	}
	if client.AccessToken != "json-token" || client.APIVersion != "2025-01" {
		t.Fatalf("unexpected JSON-configured client: %+v", client)
	}
}

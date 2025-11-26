package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ShopifyClient is a minimal client for calling the Shopify Admin REST API.
type ShopifyClient struct {
	StoreDomain string       // e.g. "lockoutsupplements.myshopify.com"
	AccessToken string       // Admin API access token
	APIVersion  string       // e.g. "2024-10"
	HTTPClient  *http.Client // can be nil; we'll default it
}

func NewShopifyClient(storeDomain, accessToken, apiVersion string) *ShopifyClient {
	return &ShopifyClient{
		StoreDomain: storeDomain,
		AccessToken: accessToken,
		APIVersion:  apiVersion,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// shopifyTransactionRequest is the payload for creating a transaction.
type shopifyTransactionRequest struct {
	Transaction shopifyTransaction `json:"transaction"`
}

type shopifyTransaction struct {
	Kind     string `json:"kind"`               // "capture" or "sale"
	Status   string `json:"status"`             // "success"
	Amount   string `json:"amount"`             // e.g. "129.99"
	Currency string `json:"currency,omitempty"` // e.g. "USD"
	Gateway  string `json:"gateway,omitempty"`  // e.g. "Green Money"
}

// MarkOrderPaid creates a successful transaction on the order.
// For manual payments this is usually enough to mark it as paid.
func (c *ShopifyClient) MarkOrderPaid(ctx context.Context, orderID int64, amount string, currency string) error {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	url := fmt.Sprintf("https://%s/admin/api/%s/orders/%d/transactions.json",
		c.StoreDomain, c.APIVersion, orderID)

	body := shopifyTransactionRequest{
		Transaction: shopifyTransaction{
			Kind:     "capture",
			Status:   "success",
			Amount:   amount,   // Keep as string so we don't fight with formatting
			Currency: currency, // optional but nice
			Gateway:  "Green Money",
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("encode transaction: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", c.AccessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read response body for debugging
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		// Log the raw body so we can see Shopify's error message
		fmt.Printf("Shopify error body: %s\n", buf.String())

		return fmt.Errorf("shopify API non-2xx: %s", resp.Status)
	}

	return nil
}

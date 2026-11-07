package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ShopifyClient is a minimal client for calling the Shopify Admin REST API.
type ShopifyClient struct {
	StoreDomain string       // e.g. "lockoutsupplements.myshopify.com"
	AccessToken string       // Admin API access token
	ClientID    string       // Optional Shopify app client ID for newer store apps
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

type shopifyGraphQLOrder struct {
	ID                     string `json:"id"`
	LegacyResourceID       string `json:"legacyResourceId"`
	DisplayFinancialStatus string `json:"displayFinancialStatus"`
	CanMarkAsPaid          bool   `json:"canMarkAsPaid"`
	FullyPaid              bool   `json:"fullyPaid"`
	TotalOutstandingSet    struct {
		ShopMoney struct {
			Amount       string `json:"amount"`
			CurrencyCode string `json:"currencyCode"`
		} `json:"shopMoney"`
	} `json:"totalOutstandingSet"`
	Transactions []struct {
		ID                   string `json:"id"`
		Kind                 string `json:"kind"`
		Status               string `json:"status"`
		Gateway              string `json:"gateway"`
		ManualPaymentGateway bool   `json:"manualPaymentGateway"`
		AmountSet            struct {
			ShopMoney struct {
				Amount       string `json:"amount"`
				CurrencyCode string `json:"currencyCode"`
			} `json:"shopMoney"`
		} `json:"amountSet"`
	} `json:"transactions"`
}

type shopifyGraphQLResponse struct {
	Data struct {
		Order      shopifyGraphQLOrder `json:"order"`
		UserErrors []struct {
			Message string `json:"message"`
		} `json:"userErrors"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *ShopifyClient) EnsureOrderPaid(ctx context.Context, orderID int64, expectedAmount, currency string) error {
	_, err := c.ensureOrderPaid(ctx, orderID, expectedAmount, currency)
	return err
}

func (c *ShopifyClient) EnsureOrderPaidWithTransaction(ctx context.Context, orderID int64, expectedAmount, currency string) (string, error) {
	return c.ensureOrderPaid(ctx, orderID, expectedAmount, currency)
}

func (c *ShopifyClient) ensureOrderPaid(ctx context.Context, orderID int64, expectedAmount, currency string) (string, error) {
	gid := fmt.Sprintf("gid://shopify/Order/%d", orderID)
	query := `query($id: ID!) { order(id: $id) { id legacyResourceId displayFinancialStatus canMarkAsPaid fullyPaid totalOutstandingSet { shopMoney { amount currencyCode } } transactions { id kind status gateway manualPaymentGateway amountSet { shopMoney { amount currencyCode } } } } }`
	var pre struct {
		Data struct {
			Order shopifyGraphQLOrder `json:"order"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.graphQL(ctx, query, map[string]any{"id": gid}, &pre); err != nil {
		return "", err
	}
	if len(pre.Errors) > 0 {
		return "", fmt.Errorf("Shopify GraphQL order query: %s", pre.Errors[0].Message)
	}
	o := pre.Data.Order
	legacyID, legacyErr := strconv.ParseInt(o.LegacyResourceID, 10, 64)
	if o.ID == "" || legacyErr != nil || legacyID != orderID {
		return "", fmt.Errorf("Shopify order identity mismatch: expected gid=%s legacy_id=%d, got gid=%s legacy_id=%s", gid, orderID, o.ID, o.LegacyResourceID)
	}
	if strings.EqualFold(o.DisplayFinancialStatus, "PAID") || o.FullyPaid {
		return "", fmt.Errorf("Shopify order already paid without attributable Burner transaction")
	}
	if !strings.EqualFold(o.TotalOutstandingSet.ShopMoney.CurrencyCode, currency) || !moneyEqual(o.TotalOutstandingSet.ShopMoney.Amount, expectedAmount) || !o.CanMarkAsPaid {
		return "", fmt.Errorf("Shopify order is not safely markable as paid")
	}
	mutation := `mutation($input: OrderMarkAsPaidInput!) { orderMarkAsPaid(input: $input) { userErrors { message } order { id legacyResourceId displayFinancialStatus fullyPaid totalOutstandingSet { shopMoney { amount currencyCode } } transactions { id kind status gateway manualPaymentGateway amountSet { shopMoney { amount currencyCode } } } } } }`
	var out struct {
		Data struct {
			Order      shopifyGraphQLOrder `json:"order"`
			UserErrors []struct {
				Message string `json:"message"`
			} `json:"userErrors"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.graphQL(ctx, mutation, map[string]any{"input": map[string]string{"id": gid}}, &out); err != nil {
		return "", err
	}
	if len(out.Errors) > 0 {
		return "", fmt.Errorf("Shopify GraphQL orderMarkAsPaid: %s", out.Errors[0].Message)
	}
	if len(out.Data.UserErrors) > 0 {
		return "", fmt.Errorf("Shopify orderMarkAsPaid: %s", out.Data.UserErrors[0].Message)
	}
	a := out.Data.Order
	postLegacyID, postLegacyErr := strconv.ParseInt(a.LegacyResourceID, 10, 64)
	if a.ID != gid || postLegacyErr != nil || postLegacyID != orderID || !a.FullyPaid || !moneyEqual(a.TotalOutstandingSet.ShopMoney.Amount, "0") {
		return "", fmt.Errorf("Shopify paid postflight failed")
	}
	var transactionID string
	for _, tx := range a.Transactions {
		if strings.EqualFold(tx.Status, "SUCCESS") && tx.ManualPaymentGateway && moneyEqual(tx.AmountSet.ShopMoney.Amount, expectedAmount) && strings.EqualFold(tx.AmountSet.ShopMoney.CurrencyCode, currency) {
			if transactionID != "" {
				return "", fmt.Errorf("multiple matching Shopify paid transactions")
			}
			transactionID = tx.ID
			break
		}
	}
	if transactionID == "" {
		return "", fmt.Errorf("Shopify paid transaction could not be attributed")
	}
	return transactionID, nil
}

func moneyEqual(a, b string) bool {
	ra, oka := new(big.Rat).SetString(strings.TrimSpace(a))
	rb, okb := new(big.Rat).SetString(strings.TrimSpace(b))
	return oka && okb && ra.Cmp(rb) == 0
}

func (c *ShopifyClient) graphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": variables})
	url := fmt.Sprintf("https://%s/admin/api/%s/graphql.json", c.StoreDomain, c.APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if requiresShopifyClientID(c.StoreDomain) {
		if strings.TrimSpace(c.ClientID) == "" {
			return fmt.Errorf("Shopify client missing client ID for shop domain %s", c.StoreDomain)
		}
		req.SetBasicAuth(c.ClientID, c.AccessToken)
	} else {
		req.Header.Set("X-Shopify-Access-Token", c.AccessToken)
	}
	h := c.HTTPClient
	if h == nil {
		h = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := h.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Shopify GraphQL non-2xx: %s", resp.Status)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
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
	if requiresShopifyClientID(c.StoreDomain) {
		if strings.TrimSpace(c.ClientID) == "" {
			return fmt.Errorf("Shopify client missing client ID for shop domain %s", c.StoreDomain)
		}
		req.SetBasicAuth(c.ClientID, c.AccessToken)
	} else {
		req.Header.Set("X-Shopify-Access-Token", c.AccessToken)
	}

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

func requiresShopifyClientID(shopDomain string) bool {
	return strings.EqualFold(strings.TrimSpace(shopDomain), "lockoutsupplements2.myshopify.com")
}

package moneyeu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	APIKey    string
	APISecret string
	HTTP      *http.Client
	Path      string
}

func NewClient(baseURL, apiKey, apiSecret string) (*Client, error) {
	// Trim trailing slash so BaseURL + "/api/..." never produces "//api/..."
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	client := Client{
		BaseURL:   baseURL,
		APIKey:    strings.TrimSpace(apiKey),
		APISecret: strings.TrimSpace(apiSecret),
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		Path:      "/api/payment/card/s2s",
	}

	if client.APIKey == "" || client.BaseURL == "" {
		return nil, fmt.Errorf("missing MoneyEU API key or base URL")
	}

	return &client, nil
}

func (c *Client) CreatePaymentS2S(ctx context.Context, req PaymentS2SRequest) (*PaymentS2SResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal card/s2s payment: %w", err)
	}

	url := c.BaseURL + c.Path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("apiKey", c.APIKey)

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("moneyEU empty response body: status=%s content-type=%q", resp.Status, resp.Header.Get("Content-Type"))
	}

	var out PaymentS2SResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w; raw=%s", err, string(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &out, fmt.Errorf("moneyEU non-2xx: %s raw=%s", resp.Status, string(raw))
	}

	return &out, nil
}

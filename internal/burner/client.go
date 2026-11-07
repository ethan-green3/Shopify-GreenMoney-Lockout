package burner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	APIVersion string
	HTTP       *http.Client
}

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string { return fmt.Sprintf("Burner API %s: %s", e.Status, e.Body) }

func (c *Client) request(ctx context.Context, method, endpoint string, body []byte, out any, idempotencyKey string) error {
	if c == nil || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.APIKey) == "" {
		return errors.New("Burner client is not configured")
	}
	base, err := url.Parse(strings.TrimSpace(c.BaseURL))
	if err != nil || base.Host == "" || (base.Scheme != "https" && os.Getenv("BURNER_ALLOW_INSECURE_HTTP") != "1") {
		return errors.New("Burner API URL must use HTTPS")
	}
	u, err := url.JoinPath(strings.TrimRight(c.BaseURL, "/"), endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(c.APIVersion) != "" {
		req.Header.Set("Burner-Version", strings.TrimSpace(c.APIVersion))
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(raw))}
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode Burner response: %w", err)
	}
	return nil
}

func (c *Client) CreateInvoice(ctx context.Context, key string, req InvoiceCreateRequest) (*Invoice, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var out Invoice
	if err := c.request(ctx, http.MethodPost, "/api/invoices", body, &out, key); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return nil, errors.New("Burner create response missing invoice id")
	}
	return &out, nil
}

func (c *Client) GetInvoice(ctx context.Context, id string) (*Invoice, error) {
	var out Invoice
	if err := c.request(ctx, http.MethodGet, "/api/invoices/"+url.PathEscape(id), nil, &out, ""); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, errors.New("Burner invoice response missing id")
	}
	return &out, nil
}

func (c *Client) ListInvoices(ctx context.Context, source string, limit, offset int) ([]Invoice, error) {
	q := url.Values{}
	if source != "" {
		q.Set("source", source)
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	var out InvoiceList
	if err := c.request(ctx, http.MethodGet, "/api/invoices?"+q.Encode(), nil, &out, ""); err != nil {
		return nil, err
	}
	return out.All(), nil
}

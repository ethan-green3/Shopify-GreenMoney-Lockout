package moneyeu

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL           string
	ProcessPaymentURL string
	APIKey            string
	APISecret         string
	HTTP              *http.Client
	Path              string
}

const moneyEUHMACServiceName = "moneyEuPayment"

func NewClient(baseURL, apiKey, apiSecret string) (*Client, error) {
	// Trim trailing slash so BaseURL + "/api/..." never produces "//api/..."
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	client := Client{
		BaseURL:   baseURL,
		APIKey:    strings.TrimSpace(apiKey),
		APISecret: strings.TrimSpace(apiSecret),
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		Path:      "/moneyEu/api/v1/processPayment",
	}

	if client.APIKey == "" || client.APISecret == "" || client.BaseURL == "" {
		return nil, fmt.Errorf("missing MoneyEU API key, secret, or base URL")
	}

	return &client, nil
}

func (c *Client) CreateCheckoutOrder(ctx context.Context, req ProcessPaymentRequest) (*ProcessPaymentResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal checkout payment: %w", err)
	}

	salt, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("generate signature salt: %w", err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := buildSignature(c.APISecret, salt, c.APIKey, timestamp, string(bodyBytes))

	endpoint := c.endpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("apiKey", c.APIKey)
	httpReq.Header.Set("salt", salt)
	httpReq.Header.Set("timestamp", timestamp)
	httpReq.Header.Set("signature", signature)
	httpReq.Header.Set("X-Flow-Type", "HPP")
	httpReq.Header.Set("flowType", "S2S")
	httpReq.Header.Set("isCheckout", "false")

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
	if !json.Valid(raw) {
		return nil, fmt.Errorf("moneyEU non-JSON response: status=%s content-type=%q raw=%s", resp.Status, resp.Header.Get("Content-Type"), string(raw))
	}

	var out ProcessPaymentResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w; raw=%s", err, string(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &out, fmt.Errorf("moneyEU non-2xx: %s raw=%s", resp.Status, string(raw))
	}
	if !out.Success {
		return &out, fmt.Errorf("moneyEU unsuccessful response: message=%q errorCode=%q", out.Message, out.ErrorCode)
	}

	return &out, nil
}

func (c *Client) endpoint() string {
	if strings.TrimSpace(c.ProcessPaymentURL) != "" {
		return strings.TrimSpace(c.ProcessPaymentURL)
	}
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/") + NormalizePath(c.Path)
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

func generateSalt() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func buildSignature(secretKey, salt, apiKey, timestamp, body string) string {
	message := moneyEUHMACServiceName + salt + apiKey + timestamp + body
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

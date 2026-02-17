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
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	APIKey    string
	APISecret string
	HTTP      *http.Client
}

func NewClient(baseURL, apiKey, apiSecret string) *Client {
	// Trim trailing slash so BaseURL + "/api/..." never produces "//api/..."
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	return &Client{
		BaseURL:   baseURL,
		APIKey:    strings.TrimSpace(apiKey),
		APISecret: strings.TrimSpace(apiSecret),
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) CreateOrderExt(ctx context.Context, req CreateOrderExtRequest) (*CreateOrderExtResponse, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("MoneyEU BaseURL is empty (check MONEYEU_BASE_URL)")
	}
	if c.APIKey == "" || c.APISecret == "" {
		return nil, fmt.Errorf("MoneyEU APIKey/APISecret missing (check env vars)")
	}

	path := "/api/createOrderExt"
	serviceName := "createOrderExt"

	// Must be compact JSON (json.Marshal is compact by default)
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal createOrderExt: %w", err)
	}
	bodyStr := string(bodyBytes)

	// Generate a per-request random salt (recommended)
	salt, err := randomSaltHex(16) // 32 hex chars
	if err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}

	// Unix seconds per docs
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// Signature per docs: Base64(HMAC-SHA256(secret, serviceName+salt+apiKey+timestamp+body))
	sig := buildSignature(serviceName, salt, c.APIKey, timestamp, bodyStr, c.APISecret)

	url := c.BaseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Required auth headers
	httpReq.Header.Set("apiKey", c.APIKey)
	httpReq.Header.Set("salt", salt)
	httpReq.Header.Set("timestamp", timestamp)
	httpReq.Header.Set("signature", sig)

	resp, err := c.HTTP.Do(httpReq)

	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("MoneyEU HTTP status=%s content-type=%q len=%d", resp.Status, resp.Header.Get("Content-Type"), len(raw))
	if len(raw) == 0 {
		// Return a clearer error with status + key headers
		return nil, fmt.Errorf("moneyEU empty response body: status=%s content-type=%q", resp.Status, resp.Header.Get("Content-Type"))
	}

	var out CreateOrderExtResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w; raw=%s", err, string(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &out, fmt.Errorf("moneyEU non-2xx: %s raw=%s", resp.Status, string(raw))
	}

	return &out, nil
}

func buildSignature(serviceName, salt, apiKey, timestamp, bodyStr, secretKey string) string {
	msg := serviceName + salt + apiKey + timestamp + bodyStr

	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(msg))
	sum := mac.Sum(nil)

	return base64.StdEncoding.EncodeToString(sum)
}

func randomSaltHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

package internal

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GreenClient holds configuration for calling Green's API.
type GreenClient struct {
	BaseURL     string       // e.g. "https://cpsandbox.com"
	ClientID    string       // GREEN_CLIENT_ID
	APIPassword string       // GREEN_API_PASSWORD
	HTTPClient  *http.Client // optional; we'll default if nil
}

// NewGreenClientFromEnv builds a GreenClient using env vars.
func NewGreenClientFromEnv() *GreenClient {
	baseURL := os.Getenv("GREEN_BASE_URL")
	if baseURL == "" {
		baseURL = "https://cpsandbox.com"
	}

	return &GreenClient{
		BaseURL:     baseURL,
		ClientID:    os.Getenv("GREEN_CLIENT_ID"),
		APIPassword: os.Getenv("GREEN_API_PASSWORD"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// InvoiceResult models the XML response from OneTimeInvoice.
type InvoiceResult struct {
	XMLName                  xml.Name `xml:"CheckProcessing InvoiceResult"`
	PaymentResult            string   `xml:"PaymentResult"`
	PaymentResultDescription string   `xml:"PaymentResultDescription"`
	InvoiceID                string   `xml:"Invoice_ID"`
	CheckID                  string   `xml:"Check_ID"`
}

// CheckStatusResult models the XML response from CheckStatus.
type CheckStatusResult struct {
	XMLName           xml.Name `xml:"CheckProcessing CheckStatusResult"`
	Result            string   `xml:"Result"`
	ResultDescription string   `xml:"ResultDescription"`
	VerifyResult      string   `xml:"VerifyResult"`
	VerifyResultDesc  string   `xml:"VerifyResultDescription"`
	VerifyOverridden  string   `xml:"VerifyOverriden"`
	Deleted           string   `xml:"Deleted"`
	DeletedDate       string   `xml:"DeletedDate"`
	Processed         string   `xml:"Processed"`
	ProcessedDate     string   `xml:"ProcessedDate"`
	Rejected          string   `xml:"Rejected"`
	RejectedDate      string   `xml:"RejectedDate"`
	CheckNumber       string   `xml:"CheckNumber"`
	CheckID           string   `xml:"Check_ID"`
}

// InvoiceStatusResult models the XML response from the InvoiceStatus endpoint.
// It lets us map from an Invoice_ID to the Check_ID that actually paid it.
type InvoiceStatusResult struct {
	XMLName                  xml.Name `xml:"InvoiceResult"`
	Result                   string   `xml:"Result"`
	ResultDescription        string   `xml:"ResultDescription"`
	PaymentResult            string   `xml:"PaymentResult"`
	PaymentResultDescription string   `xml:"PaymentResultDescription"`
	InvoiceID                string   `xml:"Invoice_ID"`
	CheckID                  string   `xml:"Check_ID"`
}

// CreateInvoice calls Green's OneTimeInvoice endpoint for a given Shopify order.
func (c *GreenClient) CreateInvoice(ctx context.Context, order ShopifyOrder) (*InvoiceResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.ClientID == "" || c.APIPassword == "" {
		return nil, fmt.Errorf("GreenClient missing ClientID or APIPassword")
	}

	customerName := buildCustomerName(order)
	email := buildCustomerEmail(order)
	if email == "" {
		return nil, fmt.Errorf("order has no email; cannot send invoice")
	}
	if customerName == "" {
		customerName = "Shopify Customer"
	}

	itemName := buildItemName(order)
	itemDescription := buildItemDescription(order)
	amount := order.TotalPrice // already a string like "7.95"
	paymentDate := buildPaymentDate()

	form := url.Values{}
	form.Set("Client_ID", c.ClientID)
	form.Set("ApiPassword", c.APIPassword)
	form.Set("CustomerName", customerName)
	form.Set("EmailAddress", email)
	form.Set("ItemName", itemName)
	form.Set("ItemDescription", itemDescription)
	form.Set("Amount", amount)
	form.Set("PaymentDate", paymentDate)
	// delimiter options are irrelevant since we parse XML
	form.Set("x_delim_data", "")
	form.Set("x_delim_char", "")

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/echeck.asmx/OneTimeInvoice"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("green OneTimeInvoice non-2xx: %s", resp.Status)
	}

	var result InvoiceResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode InvoiceResult: %w", err)
	}

	if result.InvoiceID == "" {
		return nil, fmt.Errorf("green OneTimeInvoice returned empty Invoice_ID")
	}

	return &result, nil
}

// Helper functions to build invoice fields from a ShopifyOrder.

func buildCustomerName(o ShopifyOrder) string {
	if o.BillingAddress != nil {
		fn, ln := strings.TrimSpace(o.BillingAddress.FirstName), strings.TrimSpace(o.BillingAddress.LastName)
		if fn != "" || ln != "" {
			return strings.TrimSpace(fn + " " + ln)
		}
	}
	if o.Customer != nil {
		fn, ln := strings.TrimSpace(o.Customer.FirstName), strings.TrimSpace(o.Customer.LastName)
		if fn != "" || ln != "" {
			return strings.TrimSpace(fn + " " + ln)
		}
	}
	if o.ShippingAddress != nil {
		fn, ln := strings.TrimSpace(o.ShippingAddress.FirstName), strings.TrimSpace(o.ShippingAddress.LastName)
		if fn != "" || ln != "" {
			return strings.TrimSpace(fn + " " + ln)
		}
	}
	// Fallback: use order name
	return strings.TrimSpace(o.Name)
}

func buildCustomerEmail(o ShopifyOrder) string {
	if o.Email != "" {
		return o.Email
	}
	if o.Customer != nil && o.Customer.Email != "" {
		return o.Customer.Email
	}
	return ""
}

func buildItemName(o ShopifyOrder) string {
	if o.Name != "" {
		return "Order " + o.Name
	}
	return "Shopify Order"
}

func buildItemDescription(o ShopifyOrder) string {
	if o.Name != "" && o.ID != 0 {
		return fmt.Sprintf("Lockout Supplements order %s (%d)", o.Name, o.ID)
	}
	if o.Name != "" {
		return "Lockout Supplements order " + o.Name
	}
	return "Lockout Supplements order"
}

func buildPaymentDate() string {
	// Today, formatted MM/DD/YYYY
	now := time.Now()
	return now.Format("01/02/2006")
}

// CheckStatus calls Green's CheckStatus endpoint for a given Check_ID (ChkID).
// We use this to confirm whether a debit has actually been processed/cleared
// before marking the Shopify order as paid.
func (c *GreenClient) CheckStatus(ctx context.Context, checkID string) (*CheckStatusResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.ClientID == "" || c.APIPassword == "" {
		return nil, fmt.Errorf("GreenClient missing ClientID or APIPassword")
	}

	form := url.Values{}
	form.Set("Client_ID", c.ClientID)
	form.Set("ApiPassword", c.APIPassword)
	form.Set("Check_ID", checkID)
	form.Set("x_delim_data", "")
	form.Set("x_delim_char", "")

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/echeck.asmx/CheckStatus"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("green CheckStatus non-2xx: %s", resp.Status)
	}

	var result CheckStatusResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode CheckStatusResult: %w", err)
	}

	if result.Result != "0" {
		return &result, fmt.Errorf("green CheckStatus Result=%s Description=%s", result.Result, result.ResultDescription)
	}

	return &result, nil
}

// InvoiceStatus calls Green's InvoiceStatus endpoint with an Invoice_ID and
// returns the payment status + any associated Check_ID.
//
// This is how we learn which Check_ID was created for a signed invoice.
func (c *GreenClient) InvoiceStatus(ctx context.Context, invoiceID string) (*InvoiceStatusResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.ClientID == "" || c.APIPassword == "" {
		return nil, fmt.Errorf("GreenClient missing ClientID or APIPassword")
	}

	form := url.Values{}
	form.Set("Client_ID", c.ClientID)
	form.Set("ApiPassword", c.APIPassword)
	form.Set("Invoice_ID", invoiceID)
	form.Set("x_delim_data", "")
	form.Set("x_delim_char", "")

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/echeck.asmx/InvoiceStatus"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new InvoiceStatus request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do InvoiceStatus request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("InvoiceStatus non-2xx: %s", resp.Status)
	}

	var result InvoiceStatusResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode InvoiceStatusResult: %w", err)
	}

	return &result, nil
}

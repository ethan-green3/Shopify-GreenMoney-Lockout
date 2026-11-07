package burner

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	StatusPending       = "pending"
	StatusUnderpaid     = "underpaid"
	StatusFunded        = "funded"
	StatusDeployed      = "deployed"
	StatusSwept         = "swept"
	StatusRefundPending = "refund_pending"
	StatusRefunded      = "refunded"
	StatusRefundFailed  = "refund_failed"
	StatusExpired       = "expired"
	StatusFailed        = "failed"
)

const (
	JobCreateInvoice     = "create_invoice"
	JobSendCheckoutEmail = "send_checkout_email"
	JobReconcileInvoice  = "reconcile_invoice"
	JobReconcileShopify  = "reconcile_shopify"
)

type InvoiceCreateRequest struct {
	AmountUSD string         `json:"amountUsd"`
	Memo      string         `json:"memo,omitempty"`
	Method    string         `json:"method,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Invoice struct {
	ID              string         `json:"id"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
	Status          string         `json:"status"`
	Source          string         `json:"source"`
	ReceiverAddress string         `json:"receiver_address"`
	Token           string         `json:"token"`
	Amount          string         `json:"amount"`
	ExpiresAt       string         `json:"expires_at"`
	Memo            string         `json:"memo"`
	ExternalOrderID *string        `json:"external_order_id"`
	Metadata        map[string]any `json:"metadata"`
	FundedAmount    *string        `json:"funded_amount"`
	FundedAt        *string        `json:"funded_at"`
	TxHash          *string        `json:"tx_hash"`
	RefundAddress   *string        `json:"refund_address"`
	RefundTxHash    *string        `json:"refund_tx_hash"`
	RefundedAt      *string        `json:"refunded_at"`
}

type InvoiceList struct {
	Invoices []Invoice `json:"invoices"`
	Items    []Invoice `json:"items"`
}

func (l InvoiceList) All() []Invoice {
	if len(l.Invoices) > 0 {
		return l.Invoices
	}
	return l.Items
}

func ParseUSD(value string) (int64, error) {
	s := strings.TrimSpace(value)
	if s == "" || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		return 0, fmt.Errorf("invalid USD amount %q", value)
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid USD amount %q", value)
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid USD amount %q", value)
		}
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > 2 {
		return 0, fmt.Errorf("USD amount must have at most two decimals: %q", value)
	}
	for _, r := range frac {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid USD amount %q", value)
		}
	}
	frac = frac + strings.Repeat("0", 2-len(frac))
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || major < 0 || major > (int64(^uint64(0)>>1)-99)/100 {
		return 0, fmt.Errorf("USD amount out of range %q", value)
	}
	minor, _ := strconv.ParseInt(frac, 10, 64)
	cents := major*100 + minor
	if cents <= 0 {
		return 0, fmt.Errorf("USD amount must be positive")
	}
	return cents, nil
}

func FormatUSD(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }

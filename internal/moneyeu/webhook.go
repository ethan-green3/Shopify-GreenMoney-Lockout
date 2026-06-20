package moneyeu

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lmittmann/tint"
)

type ShopifyPayer interface {
	MarkOrderPaid(ctx context.Context, orderID int64, amount string, currency string) error
}

type ShopifyResolver interface {
	ForShopDomain(shopDomain string) (ShopifyPayer, error)
}

type webhookEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Content json.RawMessage `json:"content"`
}

type webhookContentItem struct {
	ID              int64   `json:"id"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status"`
	Date            string  `json:"date"`
	IdOrderExt      string  `json:"idOrderExt"`
	OrderIDExtAlt   string  `json:"orderidext"`
	Currency        string  `json:"currency"`
	ResponseCode    string  `json:"responseCode"`
	ResponseMessage string  `json:"responseMessage"`
	Url             string  `json:"url"`
}

func (w webhookContentItem) OrderIDExt() string {
	if strings.TrimSpace(w.IdOrderExt) != "" {
		return strings.TrimSpace(w.IdOrderExt)
	}

	return strings.TrimSpace(w.OrderIDExtAlt)
}

func (w webhookContentItem) Message() string {
	if strings.TrimSpace(w.ResponseMessage) != "" {
		return strings.TrimSpace(w.ResponseMessage)
	}

	if strings.TrimSpace(w.ResponseCode) != "" {
		return strings.TrimSpace(w.ResponseCode)
	}

	return ""
}

type directMoneyEUWebhook struct {
	TransactionID    int64   `json:"transaction_id"`
	OrderIDExt       string  `json:"orderidext"`
	ResponseMessage  string  `json:"response_message"`
	PaidAmount       float64 `json:"paid_amount"`
	Currency         string  `json:"currency"`
	TransactionIDRef string  `json:"transaction_id_ref"`
	Status           string  `json:"status"`
}

func MoneyEUWebhookHandler(db *sql.DB, shopifyResolver ShopifyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if len(raw) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		log.Printf("MoneyEU webhook received: %s", string(raw))

		item, message, ok := parseMoneyEUWebhook(raw)
		if !ok || item.OrderIDExt() == "" {
			log.Printf("MoneyEU webhook missing orderidext")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		shopifyOrderID := item.OrderIDExt()
		status := strings.ToLower(strings.TrimSpace(item.Status))

		paymentInfo, err := GetMoneyEUPaymentInfoByOrderID(db, shopifyOrderID)
		if err != nil {
			log.Printf("MoneyEU webhook lookup error for order %s: %v", shopifyOrderID, err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		shopDomain := paymentInfo.ShopDomain

		_ = StoreMoneyEUWebhookEvent(db, shopDomain, shopifyOrderID, status, raw)

		if isPaidStatus(status) {
			alreadyPaid, err := IsMoneyEUShopifyMarkedPaid(db, shopDomain, shopifyOrderID)
			if err != nil {
				log.Printf("check paid error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			if alreadyPaid {
				log.Printf("MoneyEU order %s already marked paid", shopifyOrderID)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			paymentInfo, err := GetMoneyEUShopifyPaymentInfo(db, shopDomain, shopifyOrderID)
			if err != nil {
				log.Printf("lookup error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			shopify, err := shopifyResolver.ForShopDomain(paymentInfo.ShopDomain)
			if err != nil {
				log.Printf("Shopify client lookup error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			if err := shopify.MarkOrderPaid(r.Context(), paymentInfo.ShopifyNumericID, paymentInfo.AmountStr, paymentInfo.Currency); err != nil {
				log.Printf("Shopify mark paid error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			_ = MarkMoneyEUShopifyPaid(db, shopDomain, shopifyOrderID)

			slog.Info(
				"Shopify order marked paid",
				tint.Attr(10, slog.String("status", "paid")),
				slog.String("processor", "MoneyEU"),
				slog.String("shop_domain", shopDomain),
				slog.String("shopify_order_id", shopifyOrderID),
			)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		if isFailedStatus(status) {
			reason := strings.TrimSpace(message)
			if reason == "" {
				reason = item.Message()
			}
			if reason == "" {
				reason = "MoneyEU reported payment failure"
			}

			_ = MarkMoneyEUFailed(db, shopDomain, shopifyOrderID, reason)

			log.Printf("MoneyEU order %s failed (status=%s): %s", shopifyOrderID, status, reason)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		log.Printf("MoneyEU webhook received non-final status for order %s: %s", shopifyOrderID, status)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func parseMoneyEUWebhook(raw []byte) (webhookContentItem, string, bool) {
	var direct directMoneyEUWebhook
	if err := json.Unmarshal(raw, &direct); err == nil {
		if strings.TrimSpace(direct.OrderIDExt) != "" {
			return webhookContentItem{
				ID:              direct.TransactionID,
				Status:          direct.Status,
				IdOrderExt:      direct.OrderIDExt,
				Amount:          direct.PaidAmount,
				Currency:        direct.Currency,
				ResponseMessage: direct.ResponseMessage,
			}, direct.ResponseMessage, true
		}
	}

	var env webhookEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		log.Printf("MoneyEU webhook invalid JSON: %v", err)
		return webhookContentItem{}, "", false
	}

	item, ok := extractContentItem(env.Content)
	if !ok {
		return webhookContentItem{}, env.Message, false
	}

	message := strings.TrimSpace(env.Message)
	if message == "" {
		message = item.Message()
	}

	return item, message, true
}

func extractContentItem(raw json.RawMessage) (webhookContentItem, bool) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return webhookContentItem{}, false
	}

	var arr []webhookContentItem
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		if arr[0].OrderIDExt() != "" {
			return arr[0], true
		}
	}

	var obj webhookContentItem
	if err := json.Unmarshal(raw, &obj); err == nil && obj.OrderIDExt() != "" {
		return obj, true
	}

	return webhookContentItem{}, false
}

func isPaidStatus(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "captured", "paid", "completed", "success", "succeeded", "approved":
		return true
	default:
		return false
	}
}

func isFailedStatus(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "failed", "declined", "canceled", "cancelled", "error", "expired":
		return true
	default:
		return false
	}
}

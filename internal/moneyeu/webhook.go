package moneyeu

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

// This interface avoids importing internal.ShopifyClient
type ShopifyPayer interface {
	MarkOrderPaid(ctx context.Context, orderID int64, amount string, currency string) error
}

type webhookEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Content json.RawMessage `json:"content"`
}

type webhookContentItem struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	IdOrderExt string `json:"idOrderExt"`
	Url        string `json:"url"`
}

func MoneyEUWebhookHandler(db *sql.DB, shopify ShopifyPayer) http.HandlerFunc {
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

		var env webhookEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			log.Printf("MoneyEU webhook invalid JSON: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		item, ok := extractContentItem(env.Content)
		if !ok || strings.TrimSpace(item.IdOrderExt) == "" {
			log.Printf("MoneyEU webhook missing idOrderExt")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

		shopifyOrderID := strings.TrimSpace(item.IdOrderExt)
		status := strings.ToLower(strings.TrimSpace(item.Status))

		// Store webhook event
		_ = StoreMoneyEUWebhookEvent(db, shopifyOrderID, status, raw)

		if isPaidStatus(status) {

			alreadyPaid, err := IsMoneyEUShopifyMarkedPaid(db, shopifyOrderID)
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

			amountStr, currency, numericID, err := GetMoneyEUShopifyPaymentInfo(db, shopifyOrderID)
			if err != nil {
				log.Printf("lookup error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			if err := shopify.MarkOrderPaid(r.Context(), numericID, amountStr, currency); err != nil {
				log.Printf("Shopify mark paid error: %v", err)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}

			_ = MarkMoneyEUShopifyPaid(db, shopifyOrderID)

			log.Printf("MoneyEU order %s marked paid in Shopify", shopifyOrderID)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		if isFailedStatus(status) {
			reason := strings.TrimSpace(env.Message)
			if reason == "" {
				reason = "MoneyEU reported payment failure"
			}

			_ = MarkMoneyEUFailed(db, shopifyOrderID, reason)

			log.Printf("MoneyEU order %s failed (status=%s): %s", shopifyOrderID, status, reason)

			// Optional: email the customer saying payment failed and to retry
			// Optional: alert internal inbox

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}

	}
}

func extractContentItem(raw json.RawMessage) (webhookContentItem, bool) {

	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return webhookContentItem{}, false
	}

	var arr []webhookContentItem
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0], true
	}

	var obj webhookContentItem
	if err := json.Unmarshal(raw, &obj); err == nil && obj.IdOrderExt != "" {
		return obj, true
	}

	return webhookContentItem{}, false
}

func isPaidStatus(s string) bool {
	switch s {
	case "captured", "paid", "completed", "success", "succeeded":
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

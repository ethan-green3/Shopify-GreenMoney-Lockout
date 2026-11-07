package internal

import (
	"Shopify-GreenMoney-Lockout/internal/burner"
	"Shopify-GreenMoney-Lockout/internal/moneyeu"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// ShopifyOrderCreateHandlerWithBurner authenticates the shared Shopify
// endpoint once, routes Burner orders to the asynchronous Burner queue, and
// leaves legacy gateway names on their existing handlers.
func ShopifyOrderCreateHandlerWithBurner(db *sql.DB, green *GreenClient, moneySvc *moneyeu.Service, service *burner.Service, verifier *ShopifyWebhookSecrets) http.HandlerFunc {
	legacy := ShopifyOrderCreateHandlerWithAuth(db, green, moneySvc, nil)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if os.Getenv("DEBUG_LOG_WEBHOOK_BODIES") == "1" {
			log.Printf("DEBUG Shopify orders/create request body: %s", string(raw))
		}
		if strings.TrimSpace(r.Header.Get("X-Shopify-Topic")) != "orders/create" {
			http.Error(w, "unexpected topic", 400)
			return
		}
		if verifier != nil {
			if err := verifier.Verify(r, raw); err != nil {
				http.Error(w, "unauthorized", 401)
				return
			}
		}
		var order ShopifyOrder
		if err := json.Unmarshal(raw, &order); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if IsBurnerPayOrder(order) {
			if service == nil {
				http.Error(w, "Burner unavailable", 503)
				return
			}
			shopDomain, err := ExtractShopDomain(r)
			if err != nil {
				http.Error(w, "missing shop domain", 400)
				return
			}
			log.Printf("Burner order received: shop=%s order_id=%d order_name=%s amount=%s currency=%s", shopDomain, order.ID, order.Name, order.TotalPrice, order.Currency)
			_, created, err := service.AcceptShopifyOrder(r.Context(), raw, shopDomain, r.Header.Get("X-Shopify-Webhook-Id"), r.Header.Get("X-Shopify-Event-Id"))
			if err != nil {
				log.Printf("Burner order enqueue failed: %v", err)
				http.Error(w, "internal error", 500)
				return
			}
			if created {
				log.Printf("Burner order queued: shop=%s order_id=%d", shopDomain, order.ID)
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("queued"))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("duplicate"))
			}
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		legacy.ServeHTTP(w, r)
	}
}

func BurnerOrderCreateHandler(service *burner.Service, verifier *ShopifyWebhookSecrets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if verifier == nil {
			http.Error(w, "webhook authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		if topic := strings.TrimSpace(r.Header.Get("X-Shopify-Topic")); topic != "orders/create" {
			http.Error(w, "unexpected topic", 400)
			return
		}
		if err := verifier.Verify(r, raw); err != nil {
			log.Printf("Burner Shopify webhook authentication failed: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		domain, err := ExtractShopDomain(r)
		if err != nil {
			http.Error(w, "missing shop domain", 400)
			return
		}
		if service == nil {
			http.Error(w, "Burner unavailable", http.StatusServiceUnavailable)
			return
		}
		id, created, err := service.AcceptShopifyOrder(r.Context(), raw, domain, r.Header.Get("X-Shopify-Webhook-Id"), r.Header.Get("X-Shopify-Event-Id"))
		_ = id
		if err != nil {
			log.Printf("Burner order enqueue failed: %v", err)
			http.Error(w, "internal error", 500)
			return
		}
		if created {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("queued"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("duplicate"))
		}
	}
}

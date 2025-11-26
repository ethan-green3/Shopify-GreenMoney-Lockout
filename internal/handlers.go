package internal

import (
	"encoding/json"
	"log"
	"net/http"

	"database/sql"
)

// ShopifyOrderCreateHandler returns an http.HandlerFunc that handles
// the Shopify orders/create webhook and inserts a row into green_payments.
func ShopifyOrderCreateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		defer r.Body.Close()

		var order ShopifyOrder
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			log.Printf("Shopify webhook: decode error: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// If it's not a Green Money order, just ignore it.
		if !IsGreenMoneyOrder(order) {
			log.Printf("Shopify webhook: non-Green payment for order %s, ignoring", order.Name)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ignored"))
			return
		}

		// Insert row into green_payments with fake invoice/check IDs for now.
		if err := InsertGreenPayment(db, order); err != nil {
			log.Printf("Shopify webhook: insert error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("Shopify webhook: stored Green payment for order %s", order.Name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stored"))
	}
}

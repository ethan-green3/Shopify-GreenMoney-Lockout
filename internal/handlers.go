package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// ShopifyOrderCreateHandler handles the Shopify orders/create webhook,
// inserts a pending row into green_payments, and calls Green OneTimeInvoice.
func ShopifyOrderCreateHandler(db *sql.DB, green *GreenClient) http.HandlerFunc {
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

		// 1) Insert a pending payment row.
		paymentID, err := InsertPendingPayment(db, order)
		if err != nil {
			log.Printf("Shopify webhook: insert pending payment error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		log.Printf("Shopify webhook: inserted pending green_payment id=%d for order %s", paymentID, order.Name)

		// 2) Call Green OneTimeInvoice to send the invoice to the customer.
		if green == nil || green.ClientID == "" || green.APIPassword == "" {
			log.Printf("Shopify webhook: Green client not configured; skipping invoice creation")
			// Mark as invoice error so you know to fix config.
			if err := UpdatePaymentAfterInvoice(db, paymentID, "", "", StatusInvoiceError); err != nil {
				log.Printf("UpdatePaymentAfterInvoice error (no green config): %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("green_not_configured"))
			return
		}

		invResult, err := green.CreateInvoice(r.Context(), order)
		if err != nil {
			log.Printf("Shopify webhook: Green CreateInvoice error: %v", err)
			if err := UpdatePaymentAfterInvoice(db, paymentID, "", "", StatusInvoiceError); err != nil {
				log.Printf("UpdatePaymentAfterInvoice error (invoice_error): %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invoice_error"))
			return
		}

		log.Printf("Shopify webhook: Green invoice created: Invoice_ID=%s Check_ID=%s", invResult.InvoiceID, invResult.CheckID)

		// 3) Update DB with Invoice_ID and Check_ID, mark as invoice_sent.
		if err := UpdatePaymentAfterInvoice(db, paymentID, invResult.InvoiceID, invResult.CheckID, StatusInvoiceSent); err != nil {
			log.Printf("UpdatePaymentAfterInvoice error (invoice_sent): %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		log.Printf("Shopify webhook: stored Green invoice for order %s", order.Name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invoice_sent"))
	}
}

// GreenIPNHandler handles the Green Money Instant Payment Notification.
// Example request Green will send:
//
//	GET /green/ipn?ChkID=123&TransID=456
func GreenIPNHandler(db *sql.DB, shopify *ShopifyClient, green *GreenClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chkID := r.URL.Query().Get("ChkID")
		transID := r.URL.Query().Get("TransID")

		if chkID == "" {
			http.Error(w, "missing ChkID", http.StatusBadRequest)
			return
		}

		log.Printf("Green IPN received: ChkID=%s TransID=%s", chkID, transID)

		// 1) OPTIONAL SAFETY: Confirm with Green that this check has actually been processed
		//    and not rejected, so we only mark Shopify paid once funds are cleared.
		if green != nil && green.ClientID != "" && green.APIPassword != "" {
			cs, err := green.CheckStatus(r.Context(), chkID)
			if err != nil {
				log.Printf("Green CheckStatus error for ChkID=%s: %v", chkID, err)
				// You can choose to bail out here; for now we log and continue,
				// assuming Green only calls IPN when the transaction is completed.
			} else {
				processed := strings.EqualFold(cs.Processed, "true")
				rejected := strings.EqualFold(cs.Rejected, "true")

				log.Printf("Green CheckStatus: Check_ID=%s Processed=%q Rejected=%q", cs.CheckID, cs.Processed, cs.Rejected)

				if rejected {
					log.Printf("Green IPN: Check_ID=%s is rejected; not marking Shopify order as paid", cs.CheckID)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("rejected"))
					return
				}

				if !processed {
					log.Printf("Green IPN: Check_ID=%s not processed yet (Processed=%q); skipping mark-paid for now", cs.CheckID, cs.Processed)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("not_processed"))
					return
				}
			}
		} else {
			log.Printf("Green client not configured; skipping CheckStatus verification")
		}

		// 2) Look up the payment row by Check_ID (ChkID)
		payment, err := GetPaymentByCheckID(db, chkID)
		if err != nil {
			log.Printf("GetPaymentByCheckID error for ChkID=%s: %v", chkID, err)
			// For now just return 404 – in a more advanced version we could
			// call ProcessInfo and try to match by amount/email.
			http.Error(w, "payment not found", http.StatusNotFound)
			return
		}

		// 3) Call Shopify to mark the order as paid (if client is configured)
		if shopify != nil && shopify.StoreDomain != "" && shopify.AccessToken != "" {
			amountStr := fmt.Sprintf("%.2f", payment.Amount) // convert float64 to "129.99"

			if err := shopify.MarkOrderPaid(r.Context(), payment.ShopifyOrderID, amountStr, payment.Currency); err != nil {
				log.Printf("Shopify MarkOrderPaid error: %v", err)
				http.Error(w, "failed to mark Shopify paid", http.StatusBadGateway)
				return
			}
			log.Printf("Shopify: marked order %d as paid", payment.ShopifyOrderID)
		} else {
			log.Printf("Shopify client not configured; skipping Shopify mark-paid call")
		}

		// 4) Update our DB status to 'cleared' and set shopify_marked_paid_at
		if err := MarkPaymentCleared(db, chkID); err != nil {
			log.Printf("Green IPN update error: %v", err)
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}

		log.Printf("Green IPN: marked CHK %s as cleared", chkID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

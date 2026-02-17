package internal

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

const holdDuration = 24 * time.Hour // how long to wait after Processed=true before marking Shopify paid

// shouldRunGreenPoll determines whether we should run the poller logic
// at the given time. Green Money recommended polling at 8:30 AM,
// 1:30 PM, and 4:30 PM Central time.
func shouldRunGreenPoll(t time.Time) bool {
	local := t.In(time.Local)
	hour := local.Hour()
	min := local.Minute()

	switch {
	case hour == 8 && min == 30:
		return true
	case hour == 13 && min == 30: // 1:30 PM
		return true
	case hour == 16 && min == 30: // 4:30 PM
		return true
	default:
		return false
	}
}

func StartGreenPoller(ctx context.Context, db *sql.DB, green *GreenClient, shopify *ShopifyClient, interval time.Duration) {
	if green == nil || green.ClientID == "" || green.APIPassword == "" {
		log.Fatal("Green poller: Green client not configured, not starting poller")
		return
	}
	if shopify == nil || shopify.StoreDomain == "" || shopify.AccessToken == "" {
		log.Fatal("Green poller: Shopify client not configured, not starting poller")
		return
	}

	go func() {
		// Recommend setting interval to time.Minute when calling this,
		// so we check once per minute whether it's a scheduled poll time
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("Green poller: started with interval %s", interval)

		for {
			select {
			case <-ctx.Done():
				log.Println("Green poller: context canceled, stopping")
				return
			case <-ticker.C:
				now := time.Now()
				if !shouldRunGreenPoll(now) {
					// log.Println("Green Poller: Not one of the 8:30/13:30/16:30 slots.")
					continue
				}
				if err := pollOnce(ctx, db, green, shopify); err != nil {
					log.Printf("Green poller: poll error: %v", err)
				}

			}
		}
	}()
}

func pollOnce(ctx context.Context, db *sql.DB, green *GreenClient, shopify *ShopifyClient) error {
	payments, err := ListPendingInvoicePayments(db)
	if err != nil {
		return fmt.Errorf("list pending invoices: %w", err)
	}
	if len(payments) == 0 {
		log.Println("Green poller: no pending invoices")
		return nil
	}

	log.Printf("Green poller: checking %d pending invoices", len(payments))

	for _, p := range payments {
		// STEP 1: If we don't have a Check_ID yet, ask Green via InvoiceStatus.
		if p.GreenCheckID == "" {
			if p.InvoiceID == "" {
				log.Printf("Green poller: payment id=%d has no Invoice_ID or Check_ID; skipping", p.ID)
				continue
			}

			istat, err := green.InvoiceStatus(ctx, p.InvoiceID)
			if err != nil {
				log.Printf("Green poller: InvoiceStatus error for payment id=%d invoice_id=%s: %v", p.ID, p.InvoiceID, err)
				continue
			}

			// Green returns Result codes like:
			// 113 = "No payment entered."
			// 48  = "Debit entered, but not yet processed."
			// and may return CheckID="0" when no real debit exists yet.
			checkID := strings.TrimSpace(istat.CheckID)
			if checkID == "" || checkID == "0" {
				log.Printf(
					"Green poller: InvoiceStatus for invoice_id=%s has no real Check_ID yet (Result=%s %s, Check_ID=%q)",
					p.InvoiceID,
					istat.Result,
					istat.PaymentResultDescription,
					istat.CheckID,
				)
				continue
			}

			// We have a real Check_ID – persist it and update our in-memory struct.
			if err := SetCheckIDForInvoice(db, p.InvoiceID, checkID); err != nil {
				log.Printf("Green poller: SetCheckIDForInvoice error for invoice_id=%s: %v", p.InvoiceID, err)
				continue
			}
			log.Printf("Green poller: payment id=%d invoice_id=%s now has Check_ID=%s",
				p.ID, p.InvoiceID, checkID)

			p.GreenCheckID = checkID
		}

		// STEP 2: At this point we have a Check_ID; use CheckStatus to see if it has batched/processed.
		cs, err := green.CheckStatus(ctx, p.GreenCheckID)
		if err != nil {
			log.Printf("Green poller: CheckStatus error for Check_ID=%s (payment id=%d): %v", p.GreenCheckID, p.ID, err)
			continue
		}

		processed := strings.EqualFold(cs.Processed, "true")
		rejected := strings.EqualFold(cs.Rejected, "true")

		log.Printf("Green poller: CheckStatus for Check_ID=%s: Processed=%q Rejected=%q", cs.CheckID, cs.Processed, cs.Rejected)

		// STEP 2a: Handle rejected payments.
		if rejected {
			log.Printf("Green poller: payment id=%d (order %s) is rejected; marking status 'rejected'",
				p.ID, p.ShopifyOrderName)
			_, err := db.Exec(`
				UPDATE green_payments
				SET current_status = 'rejected',
				    rejected_at = NOW(),
				    updated_at = NOW(),
				    last_status_at = NOW()
				WHERE green_check_id = $1
			`, p.GreenCheckID)
			if err != nil {
				log.Printf("Green poller: failed to mark rejected payment for Check_ID=%s: %v", p.GreenCheckID, err)
			}
			continue
		}

		// STEP 2b: Not yet processed in ACH batch; nothing to do this round.
		if !processed {
			continue
		}

		// STEP 3: Processed == true && Rejected == false.
		// We DO NOT mark Shopify paid immediately. We enforce a 24h "hold" window.
		now := time.Now().UTC()

		if p.ProcessedAt == nil {
			// First time we've seen this check as processed. Record processed_at and
			// move status into a "processed_pending_lag" state so we can wait.
			_, err := db.Exec(`
				UPDATE green_payments
				SET current_status = 'processed_pending_lag',
				    processed_at = NOW(),
				    updated_at = NOW(),
				    last_status_at = NOW()
				WHERE green_check_id = $1
			`, p.GreenCheckID)
			if err != nil {
				log.Printf("Green poller: failed to set processed_pending_lag for Check_ID=%s: %v", p.GreenCheckID, err)
				continue
			}
			log.Printf("Green poller: payment id=%d (order %s) is now processed; starting 24h hold window",
				p.ID, p.ShopifyOrderName)
			continue
		}

		// We already have a processed_at; check how long it's been.
		elapsed := now.Sub(*p.ProcessedAt)
		if elapsed <= holdDuration {
			remaining := holdDuration - elapsed
			log.Printf("Green poller: payment id=%d (order %s) is in 24h hold window; %s remaining",
				p.ID, p.ShopifyOrderName, remaining.Round(time.Minute))
			continue
		}

		// STEP 4: 24h have passed since processed_at → now it's safe to mark Shopify order as paid.
		amountStr := fmt.Sprintf("%.2f", p.Amount)
		if err := shopify.MarkOrderPaid(ctx, p.ShopifyOrderID, amountStr, p.Currency); err != nil {
			log.Printf("Green poller: Shopify MarkOrderPaid error for order_id=%d: %v", p.ShopifyOrderID, err)

			// Optionally: mark as Shopify payment error so we don't retry forever.
			_, err2 := db.Exec(`
				UPDATE green_payments
				SET current_status = 'shopify_payment_error',
				    updated_at = NOW(),
				    last_status_at = NOW()
				WHERE green_check_id = $1
			`, p.GreenCheckID)
			if err2 != nil {
				log.Printf("Green poller: failed to mark shopify_payment_error for Check_ID=%s: %v", p.GreenCheckID, err2)
			}
			continue
		}
		log.Printf("Green poller: Shopify order %d marked paid by poller after 24h hold", p.ShopifyOrderID)

		// STEP 5: Update DB to 'cleared'.
		if err := MarkPaymentCleared(db, p.GreenCheckID); err != nil {
			log.Printf("Green poller: MarkPaymentCleared error for Check_ID=%s: %v", p.GreenCheckID, err)
			continue
		}
		log.Printf("Green poller: payment id=%d (Check_ID=%s) marked cleared", p.ID, p.GreenCheckID)
	}

	fmt.Println("-----------------------------")

	return nil
}

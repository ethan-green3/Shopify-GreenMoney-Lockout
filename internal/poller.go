package internal

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

func StartGreenPoller(ctx context.Context, db *sql.DB, green *GreenClient, shopify *ShopifyClient, interval time.Duration) {
	if green == nil || green.ClientID == "" || green.APIPassword == "" {
		log.Println("Green poller: Green client not configured, not starting poller")
		return
	}
	if shopify == nil || shopify.StoreDomain == "" || shopify.AccessToken == "" {
		log.Println("Green poller: Shopify client not configured, not starting poller")
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("Green poller: started with interval %s", interval)

		for {
			select {
			case <-ctx.Done():
				log.Println("Green poller: context canceled, stopping")
				return
			case <-ticker.C:
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

		// STEP 2: At this point we have a Check_ID; use CheckStatus to see if it has batched.
		cs, err := green.CheckStatus(ctx, p.GreenCheckID)
		if err != nil {
			log.Printf("Green poller: CheckStatus error for Check_ID=%s (payment id=%d): %v", p.GreenCheckID, p.ID, err)
			continue
		}

		processed := strings.EqualFold(cs.Processed, "true")
		rejected := strings.EqualFold(cs.Rejected, "true")

		log.Printf("Green poller: CheckStatus for Check_ID=%s: Processed=%q Rejected=%q", cs.CheckID, cs.Processed, cs.Rejected)

		if rejected {
			log.Printf("Green poller: payment id=%d (order %s) is rejected; consider marking status 'rejected' locally",
				p.ID, p.ShopifyOrderName)
			// Optional: add a 'rejected' status in your DB.
			continue
		}

		if !processed {
			// Not processed in ACH batch yet; nothing to do this round.
			continue
		}

		// STEP 3: If processed & not rejected, mark Shopify order as paid.
		amountStr := fmt.Sprintf("%.2f", p.Amount)
		if err := shopify.MarkOrderPaid(ctx, p.ShopifyOrderID, amountStr, p.Currency); err != nil {
			log.Printf("Green poller: Shopify MarkOrderPaid error for order_id=%d: %v", p.ShopifyOrderID, err)
			continue
		}
		log.Printf("Green poller: Shopify order %d marked paid by poller", p.ShopifyOrderID)

		// STEP 4: Update DB to 'cleared'.
		if err := MarkPaymentCleared(db, p.GreenCheckID); err != nil {
			log.Printf("Green poller: MarkPaymentCleared error for Check_ID=%s: %v", p.GreenCheckID, err)
			continue
		}
		log.Printf("Green poller: payment id=%d (Check_ID=%s) marked cleared", p.ID, p.GreenCheckID)
	}

	return nil
}

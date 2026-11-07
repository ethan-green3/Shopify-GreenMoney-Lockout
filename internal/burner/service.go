package burner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}
type ShopifyPayer interface {
	EnsureOrderPaid(ctx context.Context, orderID int64, expectedAmount, currency string) error
}
type ShopifyTransactionPayer interface {
	EnsureOrderPaidWithTransaction(ctx context.Context, orderID int64, expectedAmount, currency string) (string, error)
}
type ShopifyResolver interface {
	ForShopDomain(shopDomain string) (ShopifyPayer, error)
}

type Service struct {
	Store           *Store
	Client          *Client
	CheckoutBaseURL string
	Email           EmailSender
	Shopify         ShopifyResolver
	WorkerID        string
}

type orderPayload struct {
	ID                  int64            `json:"id"`
	Name                string           `json:"name"`
	Email               string           `json:"email"`
	Currency            string           `json:"currency"`
	TotalPrice          string           `json:"total_price"`
	OrderStatusURL      string           `json:"order_status_url"`
	Phone               string           `json:"phone"`
	PaymentGatewayNames []string         `json:"payment_gateway_names"`
	Customer            *customerPayload `json:"customer"`
	BillingAddress      *addressPayload  `json:"billing_address"`
	ShippingAddress     *addressPayload  `json:"shipping_address"`
}
type customerPayload struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}
type addressPayload struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Phone        string `json:"phone"`
	Address1     string `json:"address1"`
	Address2     string `json:"address2"`
	City         string `json:"city"`
	Province     string `json:"province"`
	ProvinceCode string `json:"province_code"`
	Zip          string `json:"zip"`
	CountryCode  string `json:"country_code"`
}

func (s *Service) AcceptShopifyOrder(ctx context.Context, raw []byte, shopDomain, webhookID, eventID string) (int64, bool, error) {
	var o orderPayload
	if err := json.Unmarshal(raw, &o); err != nil {
		return 0, false, err
	}
	cents, err := ParseUSD(o.TotalPrice)
	if err != nil {
		return 0, false, err
	}
	if o.ID <= 0 {
		return 0, false, errors.New("Burner order has invalid Shopify order ID")
	}
	if !strings.EqualFold(strings.TrimSpace(o.Currency), "USD") {
		return 0, false, fmt.Errorf("Burner only supports USD orders, got %q", o.Currency)
	}
	email := strings.TrimSpace(o.Email)
	if email == "" && o.Customer != nil {
		email = strings.TrimSpace(o.Customer.Email)
	}
	phone := strings.TrimSpace(o.Phone)
	if phone == "" && o.Customer != nil {
		phone = strings.TrimSpace(o.Customer.Phone)
	}
	addr := o.BillingAddress
	if addr == nil {
		addr = o.ShippingAddress
	}
	first, last := "", ""
	if addr != nil {
		first, last = addr.FirstName, addr.LastName
	}
	if first == "" && o.Customer != nil {
		first, last = o.Customer.FirstName, o.Customer.LastName
	}
	if email == "" {
		return 0, false, errors.New("Burner order has no customer email")
	}
	prefill := map[string]string{"email": email, "phone": phone, "first_name": first, "last_name": last, "return_url": strings.TrimSpace(o.OrderStatusURL), "source": "shopify"}
	if addr != nil {
		prefill["address_line1"] = addr.Address1
		prefill["address_line2"] = addr.Address2
		prefill["address_city"] = addr.City
		prefill["address_state"] = addr.ProvinceCode
		if prefill["address_state"] == "" {
			prefill["address_state"] = addr.Province
		}
		prefill["address_postal_code"] = addr.Zip
		prefill["address_country"] = addr.CountryCode
	}
	rawPrefill, _ := json.Marshal(prefill)
	key := fmt.Sprintf("shopify:%s:order:%d:burner:v1", strings.ToLower(strings.TrimSpace(shopDomain)), o.ID)
	return s.Store.InsertOrder(ctx, OrderInput{ShopDomain: strings.ToLower(strings.TrimSpace(shopDomain)), OrderID: o.ID, OrderName: o.Name, AmountCents: cents, Email: email, Phone: phone, Prefill: rawPrefill, IdempotencyKey: key}, webhookID, eventID)
}

func (s *Service) Run(ctx context.Context) {
	worker := s.WorkerID
	if worker == "" {
		worker = randomID()
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastSweep := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(lastSweep) >= 30*time.Second {
				if err := s.Store.EnqueueOpenReconciliations(ctx); err != nil {
					log.Printf("Burner reconciliation sweep: %v", err)
				}
				lastSweep = time.Now()
			}
			jobs, err := s.Store.Claim(ctx, worker, 4, 60*time.Second)
			if err != nil {
				log.Printf("Burner job claim: %v", err)
				continue
			}
			for _, job := range jobs {
				s.processJob(ctx, job)
			}
		}
	}
}

func (s *Service) processJob(ctx context.Context, job Job) {
	var err error
	switch job.Type {
	case JobCreateInvoice:
		err = s.createInvoice(ctx, job)
	case JobSendCheckoutEmail:
		err = s.sendEmail(ctx, job)
	case JobReconcileInvoice:
		err = s.reconcile(ctx, job)
	case JobReconcileShopify:
		err = s.reconcileShopify(ctx, job)
	default:
		err = fmt.Errorf("unsupported Burner job type %q", job.Type)
	}
	if err == nil {
		if job.Type == JobReconcileInvoice {
			_ = s.Store.Reschedule(ctx, job.ID, 5*time.Minute)
		} else {
			_ = s.Store.Complete(ctx, job.ID)
		}
		return
	}
	if errors.Is(err, errTerminal) {
		_ = s.Store.Complete(ctx, job.ID)
		return
	}
	if job.Attempts >= 5 {
		_ = s.Store.Dead(ctx, job.ID, err)
		return
	}
	_ = s.Store.Retry(ctx, job.ID, err, time.Duration(job.Attempts+1)*time.Minute)
}

func (s *Service) createInvoice(ctx context.Context, job Job) error {
	p, err := s.Store.GetPayment(ctx, job.PaymentID)
	if err != nil {
		return err
	}
	if p.InvoiceID.Valid {
		return nil
	}
	inv, err := s.Client.CreateInvoice(ctx, p.IdempotencyKey, InvoiceCreateRequest{AmountUSD: FormatUSD(p.AmountCents), Memo: "Shopify Order " + p.ShopifyOrderName, Metadata: map[string]any{"source": "shopify", "external_order_id": OrderIDString(p.ShopifyOrderID), "shop_domain": p.ShopDomain, "idempotency_key": p.IdempotencyKey}})
	if err != nil {
		// A timeout can leave creation uncertain. Before retrying, look for the
		// one invoice carrying our deterministic metadata. Never create a
		// replacement invoice when the outcome is ambiguous.
		if recovered, recoveryErr := s.recoverInvoice(ctx, p); recoveryErr == nil && recovered {
			return nil
		}
		return err
	}
	return s.finishInvoice(ctx, p, inv)
}

func (s *Service) finishInvoice(ctx context.Context, p *Payment, inv *Invoice) error {
	var fields CheckoutFields
	_ = json.Unmarshal(p.Prefill, &fields)
	checkout, err := CheckoutURL(s.CheckoutBaseURL, inv.ID, fields)
	if err != nil {
		return err
	}
	if err := s.Store.UpdateInvoice(ctx, p, inv, checkout); err != nil {
		return err
	}
	if err := s.Store.Enqueue(ctx, p.ID, JobSendCheckoutEmail, time.Now(), nil); err != nil {
		return err
	}
	return s.Store.Enqueue(ctx, p.ID, JobReconcileInvoice, time.Now().Add(time.Minute), nil)
}

func (s *Service) recoverInvoice(ctx context.Context, p *Payment) (bool, error) {
	invoices, err := s.Client.ListInvoices(ctx, "shopify", 100, 0)
	if err != nil {
		return false, err
	}
	var matches []Invoice
	for _, inv := range invoices {
		if inv.Metadata["idempotency_key"] == p.IdempotencyKey && inv.Metadata["shop_domain"] == p.ShopDomain && inv.Metadata["external_order_id"] == OrderIDString(p.ShopifyOrderID) {
			matches = append(matches, inv)
		}
	}
	if len(matches) != 1 {
		return false, nil
	}
	if err := s.finishInvoice(ctx, p, &matches[0]); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) sendEmail(ctx context.Context, job Job) error {
	p, err := s.Store.GetPayment(ctx, job.PaymentID)
	if err != nil {
		return err
	}
	if p.EmailStatus == "sent" {
		return nil
	}
	if p.CheckoutURL == "" {
		return errors.New("checkout URL is not available")
	}
	if s.Email == nil {
		return errors.New("email sender is not configured")
	}
	body := fmt.Sprintf("Thanks for your order %s. Complete payment securely here:\n%s\n\nDo not place a second order if this message is delayed; contact support for a resend.", p.ShopifyOrderName, p.CheckoutURL)
	if err := s.Email.Send(ctx, p.CustomerEmail, "Complete your Burner Pay payment", body); err != nil {
		_ = s.Store.SetEmailStatus(ctx, p.ID, "failed", err.Error())
		return err
	}
	return s.Store.SetEmailStatus(ctx, p.ID, "sent", "")
}

func (s *Service) reconcile(ctx context.Context, job Job) error {
	p, err := s.Store.GetPayment(ctx, job.PaymentID)
	if err != nil {
		return err
	}
	if !p.InvoiceID.Valid {
		return errors.New("cannot reconcile without invoice id")
	}
	inv, err := s.Client.GetInvoice(ctx, p.InvoiceID.String)
	if err != nil {
		return err
	}
	if err := s.Store.SetInvoiceStatus(ctx, p.ID, inv); err != nil {
		return err
	}
	if inv.Status == StatusSwept {
		return s.Store.Enqueue(ctx, p.ID, JobReconcileShopify, time.Now(), nil)
	}
	if inv.Status == StatusExpired || inv.Status == StatusFailed || inv.Status == StatusRefunded || inv.Status == StatusRefundFailed {
		return errTerminal
	}
	return nil
}

func (s *Service) reconcileShopify(ctx context.Context, job Job) error {
	if s.Shopify == nil {
		return errors.New("Shopify resolver is not configured")
	}
	p, err := s.Store.GetPayment(ctx, job.PaymentID)
	if err != nil {
		return err
	}
	if !p.InvoiceID.Valid {
		return errors.New("cannot reconcile Shopify without invoice id")
	}
	client, err := s.Shopify.ForShopDomain(p.ShopDomain)
	if err != nil {
		return err
	}
	var transactionID string
	if txClient, ok := client.(ShopifyTransactionPayer); ok {
		transactionID, err = txClient.EnsureOrderPaidWithTransaction(ctx, p.ShopifyOrderID, FormatUSD(p.AmountCents), p.Currency)
	} else {
		err = client.EnsureOrderPaid(ctx, p.ShopifyOrderID, FormatUSD(p.AmountCents), p.Currency)
	}
	if err != nil {
		return err
	}
	_, err = s.Store.DB.ExecContext(ctx, `UPDATE burner_payments SET shopify_sync_state='paid',shopify_marked_paid_at=NOW(),shopify_attribution='confirmed',shopify_transaction_id=NULLIF($2,''),updated_at=NOW() WHERE id=$1`, p.ID, transactionID)
	return err
}

var errTerminal = errors.New("terminal Burner invoice status")

func randomID() string { var b [8]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

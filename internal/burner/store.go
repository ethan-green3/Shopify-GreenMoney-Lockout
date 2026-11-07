package burner

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Payment struct {
	ID                   int64
	ShopDomain           string
	ShopifyOrderID       int64
	ShopifyOrderName     string
	AmountCents          int64
	Currency             string
	CustomerEmail        string
	CustomerPhone        string
	Prefill              json.RawMessage
	CheckoutURL          string
	IdempotencyKey       string
	InvoiceID            sql.NullString
	InvoiceStatus        string
	EmailStatus          string
	ShopifySyncState     string
	ShopifyTransactionID sql.NullString
}

type Job struct {
	ID        int64
	PaymentID int64
	Type      string
	State     string
	Attempts  int
	Payload   json.RawMessage
}

type Store struct{ DB *sql.DB }

func (s *Store) InsertOrder(ctx context.Context, in OrderInput, webhookID, eventID string) (int64, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	if webhookID != "" {
		res, err := tx.ExecContext(ctx, `INSERT INTO shopify_webhook_receipts(shop_domain,webhook_id,event_id,topic,shopify_order_id) VALUES($1,$2,$3,'orders/create',$4) ON CONFLICT DO NOTHING`, in.ShopDomain, webhookID, eventID, in.OrderID)
		if err != nil {
			return 0, false, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			_ = tx.Commit()
			return 0, false, nil
		}
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO burner_payments(shop_domain,shopify_order_id,shopify_order_name,amount_usd,currency,customer_email,customer_phone,checkout_prefill,idempotency_key) VALUES($1,$2,$3,$4,'USD',$5,$6,$7::jsonb,$8) ON CONFLICT(shop_domain,shopify_order_id) DO UPDATE SET updated_at=NOW() RETURNING id`, in.ShopDomain, in.OrderID, in.OrderName, FormatUSD(in.AmountCents), in.Email, in.Phone, string(in.Prefill), in.IdempotencyKey).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO burner_jobs(payment_id,job_type) SELECT $1,$2 WHERE NOT EXISTS (SELECT 1 FROM burner_jobs WHERE payment_id=$1 AND job_type=$2 AND state IN ('queued','running'))`, id, JobCreateInvoice)
	if err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (s *Store) GetPayment(ctx context.Context, id int64) (*Payment, error) {
	var p Payment
	var amount string
	var prefill []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id,shop_domain,shopify_order_id,COALESCE(shopify_order_name,''),amount_usd::text,currency,COALESCE(customer_email,''),COALESCE(customer_phone,''),COALESCE(checkout_prefill,'{}'::jsonb),COALESCE(checkout_url,''),idempotency_key,invoice_id,invoice_status,email_status,shopify_sync_state,shopify_transaction_id FROM burner_payments WHERE id=$1`, id).Scan(&p.ID, &p.ShopDomain, &p.ShopifyOrderID, &p.ShopifyOrderName, &amount, &p.Currency, &p.CustomerEmail, &p.CustomerPhone, &prefill, &p.CheckoutURL, &p.IdempotencyKey, &p.InvoiceID, &p.InvoiceStatus, &p.EmailStatus, &p.ShopifySyncState, &p.ShopifyTransactionID)
	if err != nil {
		return nil, err
	}
	cents, err := ParseUSD(amount)
	if err != nil {
		return nil, err
	}
	p.AmountCents = cents
	p.Prefill = prefill
	return &p, nil
}

func (s *Store) UpdateInvoice(ctx context.Context, p *Payment, invoice *Invoice, checkoutURL string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE burner_payments SET invoice_id=$1,invoice_status=$2,receiver_address=$3,expires_at=NULLIF($4,'')::timestamptz,invoice_amount_raw=$5,funded_amount_raw=$6,checkout_url=$7,updated_at=NOW(),last_status_at=NOW() WHERE id=$8`, invoice.ID, invoice.Status, invoice.ReceiverAddress, invoice.ExpiresAt, invoice.Amount, nullableString(invoice.FundedAmount), checkoutURL, p.ID)
	return err
}

func (s *Store) SetInvoiceStatus(ctx context.Context, paymentID int64, invoice *Invoice) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE burner_payments SET invoice_status=$1,funded_amount_raw=$2,updated_at=NOW(),last_status_at=NOW() WHERE id=$3`, invoice.Status, nullableString(invoice.FundedAmount), paymentID)
	return err
}

func (s *Store) SetEmailStatus(ctx context.Context, paymentID int64, status, message string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE burner_payments SET email_status=$1,email_error=NULLIF($2,''),email_sent_at=CASE WHEN $1='sent' THEN NOW() ELSE email_sent_at END,updated_at=NOW() WHERE id=$3`, status, message, paymentID)
	return err
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func (s *Store) Enqueue(ctx context.Context, paymentID int64, typ string, runAt time.Time, payload any) error {
	var raw []byte
	var err error
	if payload != nil {
		raw, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO burner_jobs(payment_id,job_type,run_at,payload) SELECT $1,$2,$3,$4::jsonb WHERE NOT EXISTS (SELECT 1 FROM burner_jobs WHERE payment_id=$1 AND job_type=$2 AND state IN ('queued','running'))`, paymentID, typ, runAt, nullableJSON(raw))
	return err
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func (s *Store) Claim(ctx context.Context, worker string, limit int, lease time.Duration) ([]Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,payment_id,job_type,state,attempts,COALESCE(payload,'null'::jsonb) FROM burner_jobs WHERE (state='queued' AND run_at<=NOW()) OR (state='running' AND lease_expires_at<NOW()) ORDER BY run_at,id FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.PaymentID, &j.Type, &j.State, &j.Attempts, &j.Payload); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range jobs {
		if _, err := tx.ExecContext(ctx, `UPDATE burner_jobs SET state='running',attempts=attempts+1,locked_by=$1,locked_at=NOW(),lease_expires_at=NOW()+$2::interval,updated_at=NOW() WHERE id=$3`, worker, fmt.Sprintf("%d seconds", int(lease.Seconds())), jobs[i].ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *Store) Complete(ctx context.Context, jobID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE burner_jobs SET state='succeeded',completed_at=NOW(),updated_at=NOW(),lease_expires_at=NULL WHERE id=$1`, jobID)
	return err
}
func (s *Store) Reschedule(ctx context.Context, jobID int64, delay time.Duration) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE burner_jobs SET state='queued',run_at=NOW()+$2::interval,updated_at=NOW(),lease_expires_at=NULL WHERE id=$1`, jobID, fmt.Sprintf("%d seconds", int(delay.Seconds())))
	return err
}
func (s *Store) Retry(ctx context.Context, jobID int64, err error, delay time.Duration) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE burner_jobs SET state='queued',run_at=NOW()+$2::interval,last_error=$3,updated_at=NOW(),lease_expires_at=NULL WHERE id=$1`, jobID, fmt.Sprintf("%d seconds", int(delay.Seconds())), err.Error())
	return e
}
func (s *Store) Dead(ctx context.Context, jobID int64, err error) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE burner_jobs SET state='dead',last_error=$2,updated_at=NOW(),lease_expires_at=NULL WHERE id=$1`, jobID, err.Error())
	return e
}

func (s *Store) SaveWebhook(ctx context.Context, fingerprint, event string, timestamp int64, invoiceID string, raw []byte) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO burner_webhook_events(fingerprint,event_name,invoice_id,event_timestamp,raw_payload) VALUES($1,$2,$3,$4,$5::jsonb) ON CONFLICT DO NOTHING`, fingerprint, event, invoiceID, timestamp, string(raw))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) FindPaymentByInvoiceID(ctx context.Context, invoiceID string) (*Payment, error) {
	var id int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM burner_payments WHERE invoice_id=$1`, invoiceID).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetPayment(ctx, id)
}

func (s *Store) EnqueueOpenReconciliations(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO burner_jobs(payment_id,job_type,run_at)
		SELECT p.id,$1,NOW()
		FROM burner_payments p
		WHERE p.invoice_id IS NOT NULL
		  AND p.invoice_status NOT IN ('refunded','refund_failed','expired','failed')
		  AND NOT EXISTS (SELECT 1 FROM burner_jobs j WHERE j.payment_id=p.id AND j.job_type=$1 AND j.state IN ('queued','running'))`, JobReconcileInvoice)
	return err
}

type OrderInput struct {
	ShopDomain     string
	OrderID        int64
	OrderName      string
	AmountCents    int64
	Email, Phone   string
	Prefill        []byte
	IdempotencyKey string
}

func OrderIDString(id int64) string { return strconv.FormatInt(id, 10) }

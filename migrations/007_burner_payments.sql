BEGIN;

CREATE TABLE IF NOT EXISTS burner_payments (
    id BIGSERIAL PRIMARY KEY,
    shop_domain TEXT NOT NULL,
    shopify_order_id BIGINT NOT NULL,
    shopify_order_name TEXT,
    amount_usd NUMERIC(12,2) NOT NULL CHECK (amount_usd > 0),
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    customer_email TEXT,
    customer_phone TEXT,
    checkout_prefill JSONB,
    checkout_url TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    invoice_id TEXT UNIQUE,
    invoice_status TEXT NOT NULL DEFAULT 'create_pending',
    receiver_address TEXT,
    expires_at TIMESTAMPTZ,
    invoice_amount_raw TEXT,
    funded_amount_raw TEXT,
    funded_at TIMESTAMPTZ,
    tx_hash TEXT,
    refund_address TEXT,
    refund_tx_hash TEXT,
    refunded_at TIMESTAMPTZ,
    email_sent_at TIMESTAMPTZ,
    email_status TEXT NOT NULL DEFAULT 'pending',
    email_error TEXT,
    shopify_sync_state TEXT NOT NULL DEFAULT 'not_ready',
    shopify_sync_started_at TIMESTAMPTZ,
    shopify_marked_paid_at TIMESTAMPTZ,
    shopify_transaction_id TEXT,
    shopify_attribution TEXT,
    shopify_preflight JSONB,
    needs_review_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_status_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (shop_domain, shopify_order_id)
);

CREATE INDEX IF NOT EXISTS idx_burner_payments_invoice_status
    ON burner_payments(invoice_status);
CREATE INDEX IF NOT EXISTS idx_burner_payments_shopify_sync
    ON burner_payments(shopify_sync_state);

CREATE TABLE IF NOT EXISTS burner_jobs (
    id BIGSERIAL PRIMARY KEY,
    payment_id BIGINT NOT NULL REFERENCES burner_payments(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued',
    run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INTEGER NOT NULL DEFAULT 0,
    locked_by TEXT,
    locked_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    last_error TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_burner_jobs_active_payment_type
    ON burner_jobs(payment_id, job_type)
    WHERE state IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_burner_jobs_due
    ON burner_jobs(state, run_at);

CREATE TABLE IF NOT EXISTS burner_webhook_events (
    id BIGSERIAL PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,
    event_name TEXT NOT NULL,
    invoice_id TEXT,
    event_timestamp BIGINT NOT NULL,
    raw_payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processing_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_burner_webhook_invoice
    ON burner_webhook_events(invoice_id);

CREATE TABLE IF NOT EXISTS shopify_webhook_receipts (
    id BIGSERIAL PRIMARY KEY,
    shop_domain TEXT NOT NULL,
    webhook_id TEXT NOT NULL,
    event_id TEXT,
    topic TEXT NOT NULL,
    shopify_order_id BIGINT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (shop_domain, webhook_id)
);

COMMIT;

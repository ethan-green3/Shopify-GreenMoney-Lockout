CREATE TABLE IF NOT EXISTS money_eu_payments (
  id BIGSERIAL PRIMARY KEY,

  shopify_order_id TEXT NOT NULL,
  shopify_order_name TEXT,
  amount NUMERIC(12,2) NOT NULL,
  currency TEXT NOT NULL,

  customer_email TEXT,
  customer_name TEXT,
  customer_phone TEXT,

  money_eu_order_id TEXT,
  id_order_ext TEXT,
  checkout_url TEXT,
  money_eu_status TEXT,

  current_status TEXT NOT NULL DEFAULT 'created',

  email_sent_at TIMESTAMPTZ,
  email_status TEXT,
  email_error TEXT,

  last_webhook_at TIMESTAMPTZ,
  webhook_raw JSONB,

  shopify_marked_paid_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_status_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_shopify_order_id
  ON money_eu_payments(shopify_order_id);

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_money_eu_order_id
  ON money_eu_payments(money_eu_order_id);

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_status
  ON money_eu_payments(current_status);

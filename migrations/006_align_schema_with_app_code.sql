ALTER TABLE green_payments
  ADD COLUMN IF NOT EXISTS rejected_at TIMESTAMPTZ;

ALTER TABLE green_payments
  ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS status TEXT;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS last_webhook_payload JSONB;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS failure_reason TEXT;

UPDATE money_eu_payments
SET status = COALESCE(NULLIF(status, ''), money_eu_status, current_status)
WHERE status IS NULL OR status = '';

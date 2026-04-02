BEGIN;

-- =========================
-- GREEN PAYMENTS
-- =========================

ALTER TABLE green_payments
ADD COLUMN IF NOT EXISTS shop_domain TEXT;

UPDATE green_payments
SET shop_domain = 'lockoutsupplements.myshopify.com'
WHERE shop_domain IS NULL;

CREATE INDEX IF NOT EXISTS idx_green_payments_shop_domain
  ON green_payments(shop_domain);

CREATE INDEX IF NOT EXISTS idx_green_payments_shop_order
  ON green_payments(shop_domain, shopify_order_id);


-- =========================
-- MONEY EU PAYMENTS
-- =========================

ALTER TABLE money_eu_payments
ADD COLUMN IF NOT EXISTS shop_domain TEXT;

UPDATE money_eu_payments
SET shop_domain = 'lockoutsupplements.myshopify.com'
WHERE shop_domain IS NULL;

DROP INDEX IF EXISTS idx_money_eu_payments_shopify_order_id;

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_shop_domain
  ON money_eu_payments(shop_domain);

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_shop_order
  ON money_eu_payments(shop_domain, shopify_order_id);

COMMIT;
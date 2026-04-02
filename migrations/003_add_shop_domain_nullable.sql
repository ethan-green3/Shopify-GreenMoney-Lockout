ALTER TABLE green_payments
  ADD COLUMN IF NOT EXISTS shop_domain TEXT;

ALTER TABLE money_eu_payments
  ADD COLUMN IF NOT EXISTS shop_domain TEXT;

CREATE INDEX IF NOT EXISTS idx_green_payments_shop_domain_order_id
  ON green_payments(shop_domain, shopify_order_id);

CREATE INDEX IF NOT EXISTS idx_money_eu_payments_shop_domain_order_id
  ON money_eu_payments(shop_domain, shopify_order_id);

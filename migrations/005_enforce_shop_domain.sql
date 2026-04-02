DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM green_payments WHERE shop_domain IS NULL) THEN
    RAISE EXCEPTION 'green_payments.shop_domain still has NULL rows';
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM money_eu_payments WHERE shop_domain IS NULL) THEN
    RAISE EXCEPTION 'money_eu_payments.shop_domain still has NULL rows';
  END IF;
END $$;

ALTER TABLE green_payments
  ALTER COLUMN shop_domain SET NOT NULL;

ALTER TABLE money_eu_payments
  ALTER COLUMN shop_domain SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_green_payments_shop_domain_order_id
  ON green_payments(shop_domain, shopify_order_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_money_eu_payments_shop_domain_order_id
  ON money_eu_payments(shop_domain, shopify_order_id);

UPDATE green_payments
SET shop_domain = 'lockoutsupplements.myshopify.com'
WHERE shop_domain IS NULL;

UPDATE money_eu_payments
SET shop_domain = 'lockoutsupplements.myshopify.com'
WHERE shop_domain IS NULL;

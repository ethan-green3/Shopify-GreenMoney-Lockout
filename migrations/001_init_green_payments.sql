CREATE TABLE green_payments (
    id                          BIGSERIAL PRIMARY KEY,
    shopify_order_id            TEXT NOT NULL,
    shopify_order_name          TEXT,
    amount                      NUMERIC(12,2) NOT NULL,
    currency                    CHAR(3) NOT NULL DEFAULT 'USD',

    invoice_id                  TEXT,
    green_check_id              TEXT,
    current_status              TEXT NOT NULL,
    is_cleared                  BOOLEAN NOT NULL DEFAULT FALSE,

    payment_result_code         TEXT,
    payment_result_description  TEXT,

    shopify_marked_paid_at      TIMESTAMPTZ,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_status_at              TIMESTAMPTZ
);

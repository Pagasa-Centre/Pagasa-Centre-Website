ALTER TABLE registration_groups
    DROP COLUMN IF EXISTS balance_paid_at,
    DROP COLUMN IF EXISTS invoice_due_at,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS stripe_invoice_id,
    DROP COLUMN IF EXISTS stripe_customer_id;

DROP INDEX IF EXISTS idx_reg_allocated_accommodation;

ALTER TABLE registrations
    DROP COLUMN IF EXISTS billed_stripe_price_id,
    DROP COLUMN IF EXISTS allocated_accommodation_code;

ALTER TABLE accommodation_types
    DROP COLUMN IF EXISTS stripe_price_id;

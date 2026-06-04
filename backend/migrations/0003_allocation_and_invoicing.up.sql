-- Allocation + balance invoicing (Stripe Invoices).

-- Accommodation tiers map to Stripe Price IDs (balance after deposit).
ALTER TABLE accommodation_types
    ADD COLUMN stripe_price_id TEXT;

-- Per-camper allocation (White Team) and billed Stripe Price.
ALTER TABLE registrations
    ADD COLUMN allocated_accommodation_code TEXT
        REFERENCES accommodation_types(code),
    ADD COLUMN billed_stripe_price_id TEXT;

CREATE INDEX idx_reg_allocated_accommodation
    ON registrations(allocated_accommodation_code)
    WHERE allocated_accommodation_code IS NOT NULL;

-- Group-level billing lifecycle (separate from deposit payment_status).
ALTER TABLE registration_groups
    ADD COLUMN stripe_customer_id TEXT,
    ADD COLUMN stripe_invoice_id TEXT,
    ADD COLUMN billing_status TEXT NOT NULL DEFAULT 'none'
        CHECK (billing_status IN ('none','allocated','invoiced','balance_paid','released')),
    ADD COLUMN invoice_due_at TIMESTAMPTZ,
    ADD COLUMN balance_paid_at TIMESTAMPTZ;

CREATE INDEX idx_groups_billing_status ON registration_groups(billing_status);
CREATE INDEX idx_groups_invoice_due ON registration_groups(invoice_due_at)
    WHERE billing_status = 'invoiced';

-- Placeholder Stripe Price IDs — replace via env on deploy or:
--   UPDATE accommodation_types SET stripe_price_id = 'price_...' WHERE code = 'lodge';
-- Under-3 child balance uses STRIPE_PRICE_CHILD_UNDER3 env (not a tier row).
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'lodge';
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'cabin';
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'static_caravan';
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'pod';
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'tent';
UPDATE accommodation_types SET stripe_price_id = '' WHERE code = 'child';

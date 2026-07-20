-- Coach fee tracking. Coach is charged either folded into the balance invoice
-- (new registrations) or as a separate coach-only invoice (groups already
-- invoiced/paid). Tracked in parallel columns so billing_status is untouched.
ALTER TABLE registration_groups
    ADD COLUMN coach_included_in_balance BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN stripe_coach_invoice_id   TEXT,
    ADD COLUMN coach_invoice_due_at      TIMESTAMPTZ,
    ADD COLUMN coach_fee_paid_at         TIMESTAMPTZ;

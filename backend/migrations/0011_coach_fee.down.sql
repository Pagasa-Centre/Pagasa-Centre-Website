ALTER TABLE registration_groups
    DROP COLUMN coach_included_in_balance,
    DROP COLUMN stripe_coach_invoice_id,
    DROP COLUMN coach_invoice_due_at,
    DROP COLUMN coach_fee_paid_at;

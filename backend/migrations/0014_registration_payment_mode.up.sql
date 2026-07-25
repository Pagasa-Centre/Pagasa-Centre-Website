ALTER TABLE camp_config
    ADD COLUMN registration_payment_mode TEXT NOT NULL DEFAULT 'deposit'
        CHECK (registration_payment_mode IN ('deposit','full'));

ALTER TABLE registration_groups
    ADD COLUMN paid_in_full_at_registration BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE registration_groups
    DROP COLUMN IF EXISTS paid_in_full_at_registration;

ALTER TABLE camp_config
    DROP COLUMN IF EXISTS registration_payment_mode;

DROP TABLE IF EXISTS free_codes;

ALTER TABLE registration_groups DROP CONSTRAINT registration_groups_billing_status_check;
ALTER TABLE registration_groups ADD CONSTRAINT registration_groups_billing_status_check
  CHECK (billing_status IN ('none','allocated','invoiced','balance_paid','released'));

ALTER TABLE registration_groups DROP COLUMN IF EXISTS is_free;

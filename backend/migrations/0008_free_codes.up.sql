-- Church-sponsored registration codes and billing status.

ALTER TABLE registration_groups ADD COLUMN is_free BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE registration_groups DROP CONSTRAINT registration_groups_billing_status_check;
ALTER TABLE registration_groups ADD CONSTRAINT registration_groups_billing_status_check
  CHECK (billing_status IN ('none','allocated','invoiced','balance_paid','released','free_confirmed'));

CREATE TABLE free_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL,
  note TEXT,
  used_at TIMESTAMPTZ,
  used_by_group_id UUID REFERENCES registration_groups(id) ON DELETE SET NULL,
  revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_free_codes_unused ON free_codes(code)
  WHERE used_at IS NULL AND revoked_at IS NULL;

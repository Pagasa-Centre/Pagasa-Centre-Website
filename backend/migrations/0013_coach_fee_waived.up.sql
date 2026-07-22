-- Admin can waive a group's coach fee when paid separately (e.g. Pagasa Ireland).
ALTER TABLE registration_groups
    ADD COLUMN coach_fee_waived_at TIMESTAMPTZ;

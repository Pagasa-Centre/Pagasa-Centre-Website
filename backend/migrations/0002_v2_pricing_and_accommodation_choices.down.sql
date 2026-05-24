-- Reverse of 0002_v2_pricing_and_accommodation_choices.up.sql.

-- Prices: restore variable seed rows, drop deposit.
DELETE FROM prices WHERE code = 'deposit';
INSERT INTO prices (code, display_name) VALUES
    ('full_week_adult', 'Full Week (Adult)'),
    ('full_week_child', 'Full Week (Child)'),
    ('day_pass',        'Day Pass (per day)'),
    ('tshirt_only',     'Camp T-shirt only');

-- Accommodation types: re-add nullable capacity column.
ALTER TABLE accommodation_types ADD COLUMN capacity INT;

-- Registrations: drop new columns, rename first_choice back to accommodation_code.
DROP INDEX IF EXISTS idx_reg_accommodation_second;
ALTER TABLE registrations DROP COLUMN roommate_requests;
ALTER TABLE registrations DROP COLUMN accommodation_second_choice;
ALTER INDEX idx_reg_accommodation_first RENAME TO idx_reg_accommodation;
ALTER TABLE registrations
    RENAME COLUMN accommodation_first_choice TO accommodation_code;

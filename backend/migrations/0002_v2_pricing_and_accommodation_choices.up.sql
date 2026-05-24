-- v2 of camp registration:
--   1. Accommodation becomes 1st + 2nd preference per camper (no capacity).
--   2. Pricing collapses to a flat deposit per full-week camper.
--   3. Roommate requests captured as free text (committee uses for placement).

-- 1a. Accommodation preferences ------------------------------------------------
ALTER TABLE registrations
    RENAME COLUMN accommodation_code TO accommodation_first_choice;

ALTER TABLE registrations
    ADD COLUMN accommodation_second_choice TEXT
        REFERENCES accommodation_types(code);

ALTER TABLE registrations
    ADD COLUMN roommate_requests TEXT;

ALTER INDEX idx_reg_accommodation RENAME TO idx_reg_accommodation_first;
CREATE INDEX idx_reg_accommodation_second
    ON registrations(accommodation_second_choice)
    WHERE accommodation_second_choice IS NOT NULL;

-- 1b. Drop unused capacity column ----------------------------------------------
ALTER TABLE accommodation_types DROP COLUMN capacity;

-- 2. Price catalogue: replace variable rows with a single deposit row ---------
DELETE FROM prices
 WHERE code IN ('full_week_adult', 'full_week_child', 'day_pass', 'tshirt_only');

INSERT INTO prices (code, display_name, amount_pence, currency)
VALUES ('deposit', 'Non-refundable deposit (per full-week camper)', 5000, 'GBP');

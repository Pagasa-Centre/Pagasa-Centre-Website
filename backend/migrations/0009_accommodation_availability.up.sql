-- Per-type toggle so the White Team can grey out accommodation types on the
-- public registration form. Default true preserves current behaviour.

ALTER TABLE accommodation_types
    ADD COLUMN available_for_registration BOOLEAN NOT NULL DEFAULT true;

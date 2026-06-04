-- Re-introduce accommodation capacity for the White Team allocation tracker.
-- Capacity was dropped in v2 (public registration uses preferences only, no
-- hard cap), but the admin dashboard needs it to show how many spaces remain.
-- NULL capacity means "no limit" (tents = bring your own; children share with a
-- parent and take no bed allocation).

ALTER TABLE accommodation_types
    ADD COLUMN IF NOT EXISTS capacity INT;

UPDATE accommodation_types SET capacity = 24 WHERE code = 'lodge';
UPDATE accommodation_types SET capacity = 20 WHERE code = 'cabin';
UPDATE accommodation_types SET capacity = 41 WHERE code = 'static_caravan';
UPDATE accommodation_types SET capacity = 20 WHERE code = 'pod';
-- tent + child stay NULL (no limit).

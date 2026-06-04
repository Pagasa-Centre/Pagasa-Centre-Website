-- Revert to the (incorrect) capacities seeded by migration 0004.
UPDATE accommodation_types SET capacity = 24 WHERE code = 'lodge';
UPDATE accommodation_types SET capacity = 20 WHERE code = 'cabin';
UPDATE accommodation_types SET capacity = 41 WHERE code = 'static_caravan';
UPDATE accommodation_types SET capacity = 20 WHERE code = 'pod';

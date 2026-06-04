-- Correct the accommodation capacities to match the real site inventory.
-- Capacity = number of units of that tier × beds per unit:
--   Lodge:          3 lodges  × sleeps 8           = 24
--   Cabin:          8 cabins  × sleeps 2           = 16
--   Static Caravan: 1×4 + 3×4 + 4×6                = 40
--   Pod:            10 pods   × sleeps 2           = 20
--   Tent:           unlimited (NULL)
-- Migration 0004 seeded cabin=20 and static_caravan=41, which were wrong.

UPDATE accommodation_types SET capacity = 24 WHERE code = 'lodge';
UPDATE accommodation_types SET capacity = 16 WHERE code = 'cabin';
UPDATE accommodation_types SET capacity = 40 WHERE code = 'static_caravan';
UPDATE accommodation_types SET capacity = 20 WHERE code = 'pod';
-- tent + child stay NULL (no limit).

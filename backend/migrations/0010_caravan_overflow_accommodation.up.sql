-- Caravan overflow: squeeze-in option billed like tent (2 per static caravan × 8).
INSERT INTO accommodation_types (code, display_name, capacity, sort_order, notes, stripe_price_id)
VALUES (
    'caravan_overflow',
    'Caravan - Overflow',
    16,
    6,
    'For campers happy to squeeze into a caravan with no dedicated bed space (e.g. sofa bed). Charged the same as a tent. Capacity = 2 per static caravan.',
    ''
);

UPDATE accommodation_types SET sort_order = 7 WHERE code = 'child';

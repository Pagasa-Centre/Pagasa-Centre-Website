UPDATE accommodation_types SET sort_order = 6 WHERE code = 'child';

DELETE FROM accommodation_types WHERE code = 'caravan_overflow';

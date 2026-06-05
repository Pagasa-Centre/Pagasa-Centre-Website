-- Per-unit accommodation inventory for White Team allocation.
-- Each physical unit (lodge, cabin, caravan, pod) is a row; tent/child have none.

CREATE TABLE accommodation_units (
    code               TEXT PRIMARY KEY,
    accommodation_code TEXT NOT NULL REFERENCES accommodation_types (code),
    display_name       TEXT NOT NULL,
    capacity           INT  NOT NULL,
    sort_order         INT  NOT NULL DEFAULT 0
);

-- Lodges: 3 × sleeps 8
INSERT INTO accommodation_units (code, accommodation_code, display_name, capacity, sort_order) VALUES
    ('lodge_1', 'lodge', 'Lodge 1', 8, 1),
    ('lodge_2', 'lodge', 'Lodge 2', 8, 2),
    ('lodge_3', 'lodge', 'Lodge 3', 8, 3);

-- Cabins: 8 × sleeps 2
INSERT INTO accommodation_units (code, accommodation_code, display_name, capacity, sort_order) VALUES
    ('cabin_1', 'cabin', 'Cabin 1', 2, 1),
    ('cabin_2', 'cabin', 'Cabin 2', 2, 2),
    ('cabin_3', 'cabin', 'Cabin 3', 2, 3),
    ('cabin_4', 'cabin', 'Cabin 4', 2, 4),
    ('cabin_5', 'cabin', 'Cabin 5', 2, 5),
    ('cabin_6', 'cabin', 'Cabin 6', 2, 6),
    ('cabin_7', 'cabin', 'Cabin 7', 2, 7),
    ('cabin_8', 'cabin', 'Cabin 8', 2, 8);

-- Static caravans: 4 × sleeps 4 + 4 × sleeps 6 = 40
INSERT INTO accommodation_units (code, accommodation_code, display_name, capacity, sort_order) VALUES
    ('caravan_1', 'static_caravan', 'Caravan 1', 4, 1),
    ('caravan_2', 'static_caravan', 'Caravan 2', 4, 2),
    ('caravan_3', 'static_caravan', 'Caravan 3', 4, 3),
    ('caravan_4', 'static_caravan', 'Caravan 4', 4, 4),
    ('caravan_5', 'static_caravan', 'Caravan 5', 6, 5),
    ('caravan_6', 'static_caravan', 'Caravan 6', 6, 6),
    ('caravan_7', 'static_caravan', 'Caravan 7', 6, 7),
    ('caravan_8', 'static_caravan', 'Caravan 8', 6, 8);

-- Pods: 10 × sleeps 2
INSERT INTO accommodation_units (code, accommodation_code, display_name, capacity, sort_order) VALUES
    ('pod_1', 'pod', 'Pod 1', 2, 1),
    ('pod_2', 'pod', 'Pod 2', 2, 2),
    ('pod_3', 'pod', 'Pod 3', 2, 3),
    ('pod_4', 'pod', 'Pod 4', 2, 4),
    ('pod_5', 'pod', 'Pod 5', 2, 5),
    ('pod_6', 'pod', 'Pod 6', 2, 6),
    ('pod_7', 'pod', 'Pod 7', 2, 7),
    ('pod_8', 'pod', 'Pod 8', 2, 8),
    ('pod_9', 'pod', 'Pod 9', 2, 9),
    ('pod_10', 'pod', 'Pod 10', 2, 10);

ALTER TABLE registrations
    ADD COLUMN allocated_unit_code TEXT REFERENCES accommodation_units (code);

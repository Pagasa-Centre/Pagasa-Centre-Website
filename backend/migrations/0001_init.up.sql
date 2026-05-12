CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE camp_config (
    id                 INT  PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    name               TEXT NOT NULL,
    location_name      TEXT NOT NULL,
    location_addr      TEXT NOT NULL,
    website_url        TEXT NOT NULL,
    start_date         DATE NOT NULL,
    end_date           DATE NOT NULL,
    registrations_open BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE accommodation_types (
    code         TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    capacity     INT,
    sort_order   INT  NOT NULL DEFAULT 0,
    notes        TEXT
);

CREATE TABLE prices (
    code         TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    amount_pence INT  NOT NULL DEFAULT 0,
    currency     CHAR(3) NOT NULL DEFAULT 'GBP'
);

CREATE TABLE registration_groups (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_first_name       TEXT NOT NULL,
    contact_last_name        TEXT NOT NULL,
    contact_email            TEXT NOT NULL,
    contact_phone            TEXT NOT NULL,
    payment_status           TEXT NOT NULL DEFAULT 'pending'
        CHECK (payment_status IN ('pending','paid','failed','failed_capacity','refunded','cancelled')),
    stripe_session_id        TEXT,
    stripe_payment_intent_id TEXT,
    total_amount_pence       INT  NOT NULL DEFAULT 0,
    currency                 CHAR(3) NOT NULL DEFAULT 'GBP',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at                  TIMESTAMPTZ
);
CREATE INDEX idx_groups_email ON registration_groups(contact_email);
CREATE INDEX idx_groups_status ON registration_groups(payment_status);
CREATE INDEX idx_groups_stripe_session ON registration_groups(stripe_session_id);

CREATE TABLE registrations (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id                UUID NOT NULL REFERENCES registration_groups(id) ON DELETE CASCADE,
    is_main_contact         BOOLEAN NOT NULL DEFAULT FALSE,
    first_name              TEXT NOT NULL,
    last_name               TEXT NOT NULL,
    gender                  TEXT NOT NULL CHECK (gender IN ('male','female')),
    age                     INT  NOT NULL CHECK (age > 0 AND age < 120),
    cell_leader_name        TEXT NOT NULL,
    is_cell_leader          BOOLEAN NOT NULL,
    attendance_type         TEXT NOT NULL CHECK (attendance_type IN ('full_week','day_pass')),

    shirt_size              TEXT,
    dietary_requirements    TEXT,
    needs_coach             BOOLEAN,
    accommodation_code      TEXT REFERENCES accommodation_types(code),

    day_pass_days           TEXT[],
    day_pass_tshirt_option  TEXT CHECK (day_pass_tshirt_option IN ('team_activities','tshirt_only','none')),
    day_pass_needs_catering BOOLEAN,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reg_group ON registrations(group_id);
CREATE INDEX idx_reg_accommodation ON registrations(accommodation_code) WHERE accommodation_code IS NOT NULL;

INSERT INTO camp_config VALUES
  (1, 'PC Summer Camp 2026', 'Lenchwood Trust',
   'Spitten Farm, Abbots Lench, Evesham WR11 4UP',
   'https://lenchwood.org.uk/', '2026-08-10', '2026-08-14', TRUE);

INSERT INTO accommodation_types (code, display_name, capacity, sort_order, notes) VALUES
  ('lodge',          'Lodge',          24,   1, NULL),
  ('cabin',          'Cabin',          20,   2, NULL),
  ('static_caravan', 'Static Caravan', 41,   3, NULL),
  ('pod',            'Pod',            20,   4, NULL),
  ('tent',           'Tent',           NULL, 5, 'Campers must bring own tents'),
  ('child',          'Child accommodation (sharing with parent)', NULL, 6, '12 yrs old and below');

INSERT INTO prices (code, display_name) VALUES
  ('full_week_adult', 'Full Week (Adult)'),
  ('full_week_child', 'Full Week (Child)'),
  ('day_pass',        'Day Pass (per day)'),
  ('tshirt_only',     'Camp T-shirt only');

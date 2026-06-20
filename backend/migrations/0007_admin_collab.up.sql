-- Admin collaboration: optimistic concurrency + audit log.

ALTER TABLE registration_groups
    ADD COLUMN version INT NOT NULL DEFAULT 0,
    ADD COLUMN last_action TEXT,
    ADD COLUMN last_action_by TEXT,
    ADD COLUMN last_action_at TIMESTAMPTZ;

CREATE TABLE admin_events (
    id         BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_name TEXT NOT NULL,
    action     TEXT NOT NULL,
    group_id   UUID REFERENCES registration_groups(id) ON DELETE SET NULL,
    summary    TEXT NOT NULL,
    metadata   JSONB
);

CREATE INDEX idx_admin_events_created ON admin_events(created_at DESC);

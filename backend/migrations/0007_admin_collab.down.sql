DROP TABLE IF EXISTS admin_events;

ALTER TABLE registration_groups
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS last_action,
    DROP COLUMN IF EXISTS last_action_by,
    DROP COLUMN IF EXISTS last_action_at;

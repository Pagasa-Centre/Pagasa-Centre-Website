CREATE TABLE latest_sermon (
    id           BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    video_id     TEXT NOT NULL,
    title        TEXT NOT NULL,
    published_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

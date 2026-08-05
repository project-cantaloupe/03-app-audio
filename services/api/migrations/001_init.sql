BEGIN;

CREATE TABLE IF NOT EXISTS audios (
    id UUID PRIMARY KEY,
    owner_subject TEXT NOT NULL,
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    visibility TEXT NOT NULL CHECK (visibility IN ('public', 'private')),
    status TEXT NOT NULL,
    upload_id UUID NOT NULL UNIQUE,
    source_bucket TEXT NOT NULL,
    source_key TEXT NOT NULL UNIQUE,
    source_version TEXT,
    source_checksum TEXT NOT NULL,
    source_size BIGINT NOT NULL CHECK (source_size > 0 AND source_size <= 104857600),
    source_content_type TEXT NOT NULL,
    upload_verified BOOLEAN NOT NULL DEFAULT FALSE,
    scan_status TEXT NOT NULL DEFAULT 'PENDING',
    duration_ms BIGINT,
    playback_key TEXT,
    waveform_key TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS audios_owner_created_idx
    ON audios (owner_subject, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS transcode_jobs (
    id UUID PRIMARY KEY,
    audio_id UUID NOT NULL REFERENCES audios(id),
    request_key TEXT NOT NULL,
    schema_version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL,
    target_spec JSONB NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (id, schema_version),
    UNIQUE (audio_id, request_key)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_unpublished_idx
    ON outbox_events (created_at)
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL
);

COMMIT;

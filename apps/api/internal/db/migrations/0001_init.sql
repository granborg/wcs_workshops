-- WCS Workshops initial schema.
-- Timestamps are INTEGER Unix milliseconds (UTC). Ids are UUID strings.

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    auth_uid   TEXT NOT NULL UNIQUE,            -- identity provider subject, or "dev:<email>" with the dev bypass
    email      TEXT,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'creator')),
    banner_url TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Every creator has one root directory (parent_id IS NULL). Renames keep the id, so access survives them.
CREATE TABLE directories (
    id         TEXT PRIMARY KEY,
    owner_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id  TEXT REFERENCES directories(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_directories_owner  ON directories(owner_id);
CREATE INDEX idx_directories_parent ON directories(parent_id);

CREATE TABLE videos (
    id           TEXT PRIMARY KEY,
    owner_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT NOT NULL REFERENCES directories(id) ON DELETE RESTRICT,
    title        TEXT NOT NULL,
    vimeo_id     TEXT UNIQUE,                   -- numeric id from the Vimeo uri "/videos/123"
    vimeo_hash   TEXT,                          -- unlisted hash, when privacy mode is unlisted
    status       TEXT NOT NULL DEFAULT 'uploading' CHECK (status IN ('uploading', 'processing', 'ready', 'failed')),
    duration_sec INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX idx_videos_owner     ON videos(owner_id);
CREATE INDEX idx_videos_directory ON videos(directory_id);

-- A QR code targets exactly one directory or one video.
CREATE TABLE qr_codes (
    id           TEXT PRIMARY KEY,
    code         TEXT NOT NULL UNIQUE,          -- random token encoded in the QR image
    creator_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT REFERENCES directories(id) ON DELETE CASCADE,
    video_id     TEXT REFERENCES videos(id) ON DELETE CASCADE,
    expires_at   INTEGER,
    revoked_at   INTEGER,
    created_at   INTEGER NOT NULL,
    CHECK ((directory_id IS NULL) <> (video_id IS NULL))
);
CREATE INDEX idx_qr_codes_directory ON qr_codes(directory_id);
CREATE INDEX idx_qr_codes_video     ON qr_codes(video_id);

-- Revocation sets revoked_at and keeps the row for the audit trail.
CREATE TABLE access_grants (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT REFERENCES directories(id) ON DELETE CASCADE,
    video_id     TEXT REFERENCES videos(id) ON DELETE CASCADE,
    qr_code_id   TEXT REFERENCES qr_codes(id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    revoked_at   INTEGER,
    CHECK ((directory_id IS NULL) <> (video_id IS NULL))
);
CREATE INDEX idx_access_grants_user      ON access_grants(user_id);
CREATE INDEX idx_access_grants_directory ON access_grants(directory_id);
CREATE INDEX idx_access_grants_video     ON access_grants(video_id);

CREATE TABLE devices (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    push_token TEXT NOT NULL UNIQUE,
    platform   TEXT NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_devices_user ON devices(user_id);

-- No foreign keys: audit rows must outlive the things they describe.
CREATE TABLE audit_events (
    id           TEXT PRIMARY KEY,
    at           INTEGER NOT NULL,
    type         TEXT NOT NULL,
    creator_id   TEXT NOT NULL,                 -- whose collection
    actor_id     TEXT,                          -- who performed the action
    subject_id   TEXT,                          -- who was affected
    ip           TEXT,
    directory_id TEXT,
    video_id     TEXT,
    qr_code_id   TEXT,
    detail       TEXT                           -- JSON
);
CREATE INDEX idx_audit_events_creator_at ON audit_events(creator_id, at DESC);

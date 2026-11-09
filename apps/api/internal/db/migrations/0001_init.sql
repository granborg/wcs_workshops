-- WCS Workshops initial schema.
-- Timestamps are INTEGER Unix milliseconds (UTC). Ids are UUID strings.

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    auth_uid   TEXT NOT NULL UNIQUE,            -- Firebase uid
    email      TEXT,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'creator')),
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
-- created_ip, created_at and revoked_at double as the audit trail for code generation and revocation.
CREATE TABLE qr_codes (
    id           TEXT PRIMARY KEY,
    code         TEXT NOT NULL UNIQUE,          -- random token encoded in the QR image
    creator_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT REFERENCES directories(id) ON DELETE CASCADE,
    video_id     TEXT REFERENCES videos(id) ON DELETE CASCADE,
    created_ip   TEXT,
    expires_at   INTEGER,
    revoked_at   INTEGER,
    created_at   INTEGER NOT NULL,
    CHECK ((directory_id IS NULL) <> (video_id IS NULL))
);
CREATE INDEX idx_qr_codes_creator   ON qr_codes(creator_id, created_at);
CREATE INDEX idx_qr_codes_directory ON qr_codes(directory_id);
CREATE INDEX idx_qr_codes_video     ON qr_codes(video_id);

-- Revocation sets revoked_at and keeps the row. ip, created_at and revoked_at double as the audit trail
-- for who gained and lost access. Deleting a directory or video deletes its grants and their history.
CREATE TABLE access_grants (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    creator_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT REFERENCES directories(id) ON DELETE CASCADE,
    video_id     TEXT REFERENCES videos(id) ON DELETE CASCADE,
    qr_code_id   TEXT REFERENCES qr_codes(id) ON DELETE SET NULL,
    ip           TEXT,
    created_at   INTEGER NOT NULL,
    revoked_at   INTEGER,
    CHECK ((directory_id IS NULL) <> (video_id IS NULL))
);
CREATE INDEX idx_access_grants_user      ON access_grants(user_id, creator_id);
CREATE INDEX idx_access_grants_creator   ON access_grants(creator_id, created_at);
CREATE INDEX idx_access_grants_directory ON access_grants(directory_id);
CREATE INDEX idx_access_grants_video     ON access_grants(video_id);

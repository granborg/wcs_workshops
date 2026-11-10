-- Collection admins: another creator who may manage the videos, share codes and access of a folder
-- and everything under it. An admin on the root folder manages the whole collection.
CREATE TABLE collection_admins (
    id           TEXT PRIMARY KEY,
    owner_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,       -- the collection owner
    directory_id TEXT NOT NULL REFERENCES directories(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,       -- the admin
    created_at   INTEGER NOT NULL,
    UNIQUE (directory_id, user_id)
);
CREATE INDEX idx_collection_admins_user  ON collection_admins(user_id, owner_id);
CREATE INDEX idx_collection_admins_owner ON collection_admins(owner_id);

-- Upload links let any signed-in user upload the file of one video that has no file yet.
-- Revocation sets revoked_at and keeps the row. created_at, used_at and revoked_at are the audit trail.
CREATE TABLE upload_links (
    id         TEXT PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,                                         -- random token in the link
    owner_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,         -- the collection owner
    video_id   TEXT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    created_ip TEXT,
    used_by    TEXT REFERENCES users(id) ON DELETE SET NULL,                 -- the last user who started an upload
    used_ip    TEXT,
    used_at    INTEGER,
    revoked_at INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_upload_links_video ON upload_links(video_id);
CREATE INDEX idx_upload_links_owner ON upload_links(owner_id, created_at);

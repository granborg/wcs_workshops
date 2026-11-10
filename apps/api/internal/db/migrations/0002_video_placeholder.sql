-- Add the 'placeholder' video status: a video row with no file yet. The creator uploads the file later.
-- SQLite cannot change a CHECK constraint in place, so rebuild the table (Migrate turns foreign keys off).

CREATE TABLE videos_new (
    id           TEXT PRIMARY KEY,
    owner_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    directory_id TEXT NOT NULL REFERENCES directories(id) ON DELETE RESTRICT,
    title        TEXT NOT NULL,
    vimeo_id     TEXT UNIQUE,                   -- numeric id from the Vimeo uri "/videos/123"; NULL for a placeholder
    vimeo_hash   TEXT,                          -- unlisted hash, when privacy mode is unlisted
    status       TEXT NOT NULL DEFAULT 'uploading'
                 CHECK (status IN ('placeholder', 'uploading', 'processing', 'ready', 'failed')),
    duration_sec INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

INSERT INTO videos_new (id, owner_id, directory_id, title, vimeo_id, vimeo_hash, status, duration_sec, created_at, updated_at)
SELECT id, owner_id, directory_id, title, vimeo_id, vimeo_hash, status, duration_sec, created_at, updated_at FROM videos;

DROP TABLE videos;
ALTER TABLE videos_new RENAME TO videos;

CREATE INDEX idx_videos_owner     ON videos(owner_id);
CREATE INDEX idx_videos_directory ON videos(directory_id);

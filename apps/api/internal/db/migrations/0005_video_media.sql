-- Facts about the video file that Vimeo knows when processing is done. The app asks Vimeo once and keeps them.
ALTER TABLE videos ADD COLUMN width INTEGER;            -- pixels; the player uses width and height for its shape
ALTER TABLE videos ADD COLUMN height INTEGER;
ALTER TABLE videos ADD COLUMN thumbnail_url TEXT;       -- picture on Vimeo's image server, about 640 pixels wide
ALTER TABLE videos ADD COLUMN media_checked_at INTEGER; -- the last time the app asked Vimeo for them (Unix ms)

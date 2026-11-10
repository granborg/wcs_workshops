-- Frame size and thumbnail of the uploaded file, read from Vimeo once the video is processed.
-- The app sizes the player from width and height, so a portrait video is not letterboxed in a 16:9 frame.
ALTER TABLE videos ADD COLUMN width INTEGER;
ALTER TABLE videos ADD COLUMN height INTEGER;
ALTER TABLE videos ADD COLUMN thumbnail_url TEXT;

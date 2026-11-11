-- The date of the video, as the creator enters it (for example, the day of the workshop). Format YYYY-MM-DD.
-- The app sorts videos by this date. Existing videos get the UTC date of their upload; the creator can change it.
ALTER TABLE videos ADD COLUMN video_date TEXT;

UPDATE videos SET video_date = strftime('%Y-%m-%d', created_at / 1000, 'unixepoch');

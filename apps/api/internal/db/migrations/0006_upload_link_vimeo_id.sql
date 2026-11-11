-- The Vimeo upload that the link's user started. Completing the link gives access only when the video still
-- points at that upload: an upload that the owner or someone else replaced does not count.
ALTER TABLE upload_links ADD COLUMN used_vimeo_id TEXT;

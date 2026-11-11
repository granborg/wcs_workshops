package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const videoCols = `id, owner_id, directory_id, title, vimeo_id, vimeo_hash, status, duration_sec, video_date,
	width, height, thumbnail_url, media_checked_at, created_at, updated_at`

func scanVideo(r scanner) (*model.Video, error) {
	var v model.Video
	var vimeoID, vimeoHash, videoDate, thumbnail sql.NullString
	var duration, width, height, checked sql.NullInt64
	var created, updated int64
	if err := r.Scan(&v.ID, &v.OwnerID, &v.DirectoryID, &v.Title, &vimeoID, &vimeoHash, &v.Status, &duration, &videoDate,
		&width, &height, &thumbnail, &checked, &created, &updated); err != nil {
		return nil, err
	}
	v.Width = ptrInt(width)
	v.Height = ptrInt(height)
	v.ThumbnailURL = ptrStr(thumbnail)
	v.MediaCheckedAt = ptrTime(checked)
	v.VimeoID = ptrStr(vimeoID)
	v.VimeoHash = ptrStr(vimeoHash)
	v.DurationSec = ptrInt(duration)
	v.VideoDate = ptrStr(videoDate)
	v.CreatedAt = fromMs(created)
	v.UpdatedAt = fromMs(updated)
	return &v, nil
}

func (s *Store) GetVideo(ctx context.Context, id string) (*model.Video, error) {
	v, err := scanVideo(s.db.QueryRowContext(ctx, `SELECT `+videoCols+` FROM videos WHERE id = ?`, id))
	return v, wrapNotFound(err)
}

func (s *Store) ListVideos(ctx context.Context, ownerID string) ([]model.Video, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+videoCols+` FROM videos WHERE owner_id = ? ORDER BY title COLLATE NOCASE`, ownerID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanVideo)
}

func (s *Store) CountVideos(ctx context.Context, ownerID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos WHERE owner_id = ?`, ownerID).Scan(&n)
	return n, err
}

// CreateVideo adds a video row. Without a Vimeo id, the row is a placeholder for a file that comes later.
// videoDate is YYYY-MM-DD or nil.
func (s *Store) CreateVideo(ctx context.Context, ownerID, directoryID, title string, videoDate, vimeoID, vimeoHash *string) (*model.Video, error) {
	t := now()
	status := model.StatusUploading
	if vimeoID == nil {
		status = model.StatusPlaceholder
	}
	v := &model.Video{
		ID: newID(), OwnerID: ownerID, DirectoryID: directoryID, Title: title,
		VimeoID: vimeoID, VimeoHash: vimeoHash, Status: status, VideoDate: videoDate, CreatedAt: t, UpdatedAt: t,
	}
	err := s.exec(ctx, `INSERT INTO videos (`+videoCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL, NULL, NULL, NULL, ?, ?)`,
		v.ID, v.OwnerID, v.DirectoryID, v.Title, nullStr(v.VimeoID), nullStr(v.VimeoHash), v.Status, nullStr(v.VideoDate), toMs(t), toMs(t))
	return v, err
}

// UpdateVideo retitles, moves and/or changes the date. Nil fields are left alone. An empty videoDate clears it.
func (s *Store) UpdateVideo(ctx context.Context, id string, title, directoryID, videoDate *string) (*model.Video, error) {
	sets := []string{"updated_at = ?"}
	args := []any{toMs(now())}
	if title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *title)
	}
	if directoryID != nil {
		sets = append(sets, "directory_id = ?")
		args = append(args, *directoryID)
	}
	if videoDate != nil {
		sets = append(sets, "video_date = ?")
		if *videoDate == "" {
			args = append(args, nil)
		} else {
			args = append(args, *videoDate)
		}
	}
	args = append(args, id)
	if err := s.exec(ctx, `UPDATE videos SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		return nil, err
	}
	return s.GetVideo(ctx, id)
}

// UpdateVideoFromVimeo stores what Vimeo reports about the video. Nil values keep what is stored.
func (s *Store) UpdateVideoFromVimeo(ctx context.Context, id, status string, durationSec, width, height *int, thumbnailURL *string) (*model.Video, error) {
	t := toMs(now())
	if err := s.exec(ctx,
		`UPDATE videos SET status = ?, duration_sec = COALESCE(?, duration_sec), width = COALESCE(?, width),
		 height = COALESCE(?, height), thumbnail_url = COALESCE(?, thumbnail_url), media_checked_at = ?, updated_at = ?
		 WHERE id = ?`,
		status, nullInt(durationSec), nullInt(width), nullInt(height), nullStr(thumbnailURL), t, t, id); err != nil {
		return nil, err
	}
	return s.GetVideo(ctx, id)
}

// AttachUpload points an existing video at a new Vimeo upload and resets its status to uploading.
// The size and thumbnail of a previous file are cleared.
func (s *Store) AttachUpload(ctx context.Context, id, vimeoID string, vimeoHash *string) (*model.Video, error) {
	if err := s.exec(ctx,
		`UPDATE videos SET vimeo_id = ?, vimeo_hash = ?, status = ?, duration_sec = NULL, width = NULL, height = NULL,
		 thumbnail_url = NULL, media_checked_at = NULL, updated_at = ? WHERE id = ?`,
		vimeoID, nullStr(vimeoHash), model.StatusUploading, toMs(now()), id); err != nil {
		return nil, err
	}
	return s.GetVideo(ctx, id)
}

func (s *Store) MoveVideos(ctx context.Context, ids []string, directoryID string) error {
	if len(ids) == 0 {
		return nil
	}
	args := append([]any{directoryID, toMs(now())}, toAny(ids)...)
	return s.exec(ctx,
		`UPDATE videos SET directory_id = ?, updated_at = ? WHERE id IN (`+placeholders(len(ids))+`)`, args...)
}

func (s *Store) DeleteVideo(ctx context.Context, id string) error {
	return s.exec(ctx, `DELETE FROM videos WHERE id = ?`, id)
}

package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const videoCols = `id, owner_id, directory_id, title, vimeo_id, vimeo_hash, status, duration_sec, created_at, updated_at`

func scanVideo(r scanner) (*model.Video, error) {
	var v model.Video
	var vimeoID, vimeoHash sql.NullString
	var duration sql.NullInt64
	var created, updated int64
	if err := r.Scan(&v.ID, &v.OwnerID, &v.DirectoryID, &v.Title, &vimeoID, &vimeoHash, &v.Status, &duration, &created, &updated); err != nil {
		return nil, err
	}
	v.VimeoID = ptrStr(vimeoID)
	v.VimeoHash = ptrStr(vimeoHash)
	v.DurationSec = ptrInt(duration)
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
func (s *Store) CreateVideo(ctx context.Context, ownerID, directoryID, title string, vimeoID, vimeoHash *string) (*model.Video, error) {
	t := now()
	status := model.StatusUploading
	if vimeoID == nil {
		status = model.StatusPlaceholder
	}
	v := &model.Video{
		ID: newID(), OwnerID: ownerID, DirectoryID: directoryID, Title: title,
		VimeoID: vimeoID, VimeoHash: vimeoHash, Status: status, CreatedAt: t, UpdatedAt: t,
	}
	err := s.exec(ctx, `INSERT INTO videos (`+videoCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		v.ID, v.OwnerID, v.DirectoryID, v.Title, nullStr(v.VimeoID), nullStr(v.VimeoHash), v.Status, toMs(t), toMs(t))
	return v, err
}

// UpdateVideo retitles and/or moves. Nil fields are left alone.
func (s *Store) UpdateVideo(ctx context.Context, id string, title, directoryID *string) (*model.Video, error) {
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
	args = append(args, id)
	if err := s.exec(ctx, `UPDATE videos SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		return nil, err
	}
	return s.GetVideo(ctx, id)
}

func (s *Store) UpdateVideoStatus(ctx context.Context, id, status string, durationSec *int) (*model.Video, error) {
	var dur any
	if durationSec != nil {
		dur = *durationSec
	}
	if err := s.exec(ctx,
		`UPDATE videos SET status = ?, duration_sec = COALESCE(?, duration_sec), updated_at = ? WHERE id = ?`,
		status, dur, toMs(now()), id); err != nil {
		return nil, err
	}
	return s.GetVideo(ctx, id)
}

// AttachUpload points an existing video at a new Vimeo upload and resets its status to uploading.
func (s *Store) AttachUpload(ctx context.Context, id, vimeoID string, vimeoHash *string) (*model.Video, error) {
	if err := s.exec(ctx,
		`UPDATE videos SET vimeo_id = ?, vimeo_hash = ?, status = ?, duration_sec = NULL, updated_at = ? WHERE id = ?`,
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

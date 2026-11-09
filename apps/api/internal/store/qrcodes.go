package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const qrCols = `q.id, q.code, q.creator_id, q.directory_id, q.video_id, q.expires_at, q.revoked_at, q.created_at,
	(SELECT COUNT(*) FROM access_grants g WHERE g.qr_code_id = q.id)`

func scanQrCode(r scanner) (*model.QrCode, error) {
	var q model.QrCode
	var dir, vid sql.NullString
	var expires, revoked sql.NullInt64
	var created int64
	if err := r.Scan(&q.ID, &q.Code, &q.CreatorID, &dir, &vid, &expires, &revoked, &created, &q.Redemptions); err != nil {
		return nil, err
	}
	q.DirectoryID = ptrStr(dir)
	q.VideoID = ptrStr(vid)
	q.ExpiresAt = ptrTime(expires)
	q.RevokedAt = ptrTime(revoked)
	q.CreatedAt = fromMs(created)
	return &q, nil
}

func (s *Store) GetQrCode(ctx context.Context, id string) (*model.QrCode, error) {
	q, err := scanQrCode(s.db.QueryRowContext(ctx, `SELECT `+qrCols+` FROM qr_codes q WHERE q.id = ?`, id))
	return q, wrapNotFound(err)
}

func (s *Store) GetQrCodeByCode(ctx context.Context, code string) (*model.QrCode, error) {
	q, err := scanQrCode(s.db.QueryRowContext(ctx, `SELECT `+qrCols+` FROM qr_codes q WHERE q.code = ?`, code))
	return q, wrapNotFound(err)
}

// ListQrCodes returns the codes on one directory or one video, newest first.
func (s *Store) ListQrCodes(ctx context.Context, creatorID string, directoryID, videoID *string) ([]model.QrCode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+qrCols+` FROM qr_codes q
		 WHERE q.creator_id = ? AND q.directory_id IS ? AND q.video_id IS ?
		 ORDER BY q.created_at DESC`,
		creatorID, nullStr(directoryID), nullStr(videoID))
	if err != nil {
		return nil, err
	}
	return collect(rows, scanQrCode)
}

func (s *Store) CreateQrCode(ctx context.Context, code, creatorID string, directoryID, videoID *string, expiresAt *time.Time) (*model.QrCode, error) {
	id := newID()
	if err := s.exec(ctx,
		`INSERT INTO qr_codes (id, code, creator_id, directory_id, video_id, expires_at, revoked_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, ?)`,
		id, code, creatorID, nullStr(directoryID), nullStr(videoID), nullMs(expiresAt), toMs(now())); err != nil {
		return nil, err
	}
	return s.GetQrCode(ctx, id)
}

func (s *Store) RevokeQrCode(ctx context.Context, id string, at time.Time) error {
	return s.exec(ctx, `UPDATE qr_codes SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, toMs(at), id)
}

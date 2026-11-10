package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const uploadLinkCols = `l.id, l.code, l.owner_id, l.video_id, l.created_ip, l.used_by, l.used_ip, l.used_at, l.revoked_at, l.created_at,
	(SELECT name FROM users u WHERE u.id = l.used_by)`

func scanUploadLink(r scanner) (*model.UploadLink, error) {
	var l model.UploadLink
	var createdIP, usedBy, usedIP, usedByName sql.NullString
	var usedAt, revoked sql.NullInt64
	var created int64
	if err := r.Scan(&l.ID, &l.Code, &l.OwnerID, &l.VideoID, &createdIP, &usedBy, &usedIP, &usedAt, &revoked, &created, &usedByName); err != nil {
		return nil, err
	}
	l.CreatedIP = ptrStr(createdIP)
	l.UsedBy = ptrStr(usedBy)
	l.UsedByName = ptrStr(usedByName)
	l.UsedIP = ptrStr(usedIP)
	l.UsedAt = ptrTime(usedAt)
	l.RevokedAt = ptrTime(revoked)
	l.CreatedAt = fromMs(created)
	return &l, nil
}

func (s *Store) GetUploadLink(ctx context.Context, id string) (*model.UploadLink, error) {
	l, err := scanUploadLink(s.db.QueryRowContext(ctx, `SELECT `+uploadLinkCols+` FROM upload_links l WHERE l.id = ?`, id))
	return l, wrapNotFound(err)
}

func (s *Store) GetUploadLinkByCode(ctx context.Context, code string) (*model.UploadLink, error) {
	l, err := scanUploadLink(s.db.QueryRowContext(ctx, `SELECT `+uploadLinkCols+` FROM upload_links l WHERE l.code = ?`, code))
	return l, wrapNotFound(err)
}

// ListUploadLinks returns the links of one video, newest first.
func (s *Store) ListUploadLinks(ctx context.Context, videoID string) ([]model.UploadLink, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+uploadLinkCols+` FROM upload_links l WHERE l.video_id = ? ORDER BY l.created_at DESC`, videoID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanUploadLink)
}

func (s *Store) CreateUploadLink(ctx context.Context, code, ownerID, videoID string, createdIP *string) (*model.UploadLink, error) {
	id := newID()
	if err := s.exec(ctx,
		`INSERT INTO upload_links (id, code, owner_id, video_id, created_ip, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, code, ownerID, videoID, nullStr(createdIP), toMs(now())); err != nil {
		return nil, err
	}
	return s.GetUploadLink(ctx, id)
}

// MarkUploadLinkUsed records who started an upload with the link, and when.
func (s *Store) MarkUploadLinkUsed(ctx context.Context, id, userID string, ip *string) error {
	return s.exec(ctx, `UPDATE upload_links SET used_by = ?, used_ip = ?, used_at = ? WHERE id = ?`,
		userID, nullStr(ip), toMs(now()), id)
}

func (s *Store) RevokeUploadLink(ctx context.Context, id string, at time.Time) error {
	return s.exec(ctx, `UPDATE upload_links SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, toMs(at), id)
}

package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const grantCols = `g.id, g.user_id, g.creator_id, g.directory_id, g.video_id, g.qr_code_id, g.ip, g.created_at, g.revoked_at`

func scanGrantInto(r scanner, g *model.AccessGrant, extra ...any) error {
	var dir, vid, qr, ip sql.NullString
	var revoked sql.NullInt64
	var created int64
	dest := []any{&g.ID, &g.UserID, &g.CreatorID, &dir, &vid, &qr, &ip, &created, &revoked}
	dest = append(dest, extra...)
	if err := r.Scan(dest...); err != nil {
		return err
	}
	g.DirectoryID = ptrStr(dir)
	g.VideoID = ptrStr(vid)
	g.QrCodeID = ptrStr(qr)
	g.IP = ptrStr(ip)
	g.CreatedAt = fromMs(created)
	g.RevokedAt = ptrTime(revoked)
	return nil
}

func scanGrant(r scanner) (*model.AccessGrant, error) {
	var g model.AccessGrant
	if err := scanGrantInto(r, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// ActiveGrantRefs returns the directory ids and video ids the user holds active grants on in one creator's collection.
func (s *Store) ActiveGrantRefs(ctx context.Context, userID, creatorID string) (dirIDs, videoIDs []string, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT directory_id, video_id FROM access_grants WHERE user_id = ? AND creator_id = ? AND revoked_at IS NULL`,
		userID, creatorID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	dirIDs, videoIDs = []string{}, []string{}
	for rows.Next() {
		var dir, vid sql.NullString
		if err := rows.Scan(&dir, &vid); err != nil {
			return nil, nil, err
		}
		if dir.Valid {
			dirIDs = append(dirIDs, dir.String)
		}
		if vid.Valid {
			videoIDs = append(videoIDs, vid.String)
		}
	}
	return dirIDs, videoIDs, rows.Err()
}

// CreatorsSharedWith returns the ids of creators who have granted the user active access to anything.
func (s *Store) CreatorsSharedWith(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT creator_id FROM access_grants WHERE user_id = ? AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// FindActiveGrant returns the user's active grant on exactly this target, or nil.
func (s *Store) FindActiveGrant(ctx context.Context, userID string, directoryID, videoID *string) (*model.AccessGrant, error) {
	g, err := scanGrant(s.db.QueryRowContext(ctx,
		`SELECT `+grantCols+` FROM access_grants g
		 WHERE g.user_id = ? AND g.revoked_at IS NULL AND g.directory_id IS ? AND g.video_id IS ?
		 LIMIT 1`,
		userID, nullStr(directoryID), nullStr(videoID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return g, err
}

func (s *Store) CreateGrant(ctx context.Context, userID, creatorID string, directoryID, videoID, qrCodeID, ip *string) (*model.AccessGrant, error) {
	g := &model.AccessGrant{
		ID: newID(), UserID: userID, CreatorID: creatorID, DirectoryID: directoryID, VideoID: videoID,
		QrCodeID: qrCodeID, IP: ip, CreatedAt: now(),
	}
	err := s.exec(ctx,
		`INSERT INTO access_grants (id, user_id, creator_id, directory_id, video_id, qr_code_id, ip, created_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		g.ID, g.UserID, g.CreatorID, nullStr(g.DirectoryID), nullStr(g.VideoID), nullStr(g.QrCodeID), nullStr(g.IP), toMs(g.CreatedAt))
	return g, err
}

// GrantWithUser is a grant plus the grantee's display name.
type GrantWithUser struct {
	Grant    model.AccessGrant
	UserName string
}

// ListActiveGrantsOn returns active grants placed directly on one directory or one video of a creator.
func (s *Store) ListActiveGrantsOn(ctx context.Context, creatorID string, directoryID, videoID *string) ([]GrantWithUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+grantCols+`, u.name
		 FROM access_grants g JOIN users u ON u.id = g.user_id
		 WHERE g.creator_id = ? AND g.revoked_at IS NULL AND g.directory_id IS ? AND g.video_id IS ?
		 ORDER BY g.created_at DESC`,
		creatorID, nullStr(directoryID), nullStr(videoID))
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r scanner) (*GrantWithUser, error) {
		var gw GrantWithUser
		if err := scanGrantInto(r, &gw.Grant, &gw.UserName); err != nil {
			return nil, err
		}
		return &gw, nil
	})
}

func (s *Store) GetGrant(ctx context.Context, id string) (*model.AccessGrant, error) {
	g, err := scanGrant(s.db.QueryRowContext(ctx, `SELECT `+grantCols+` FROM access_grants g WHERE g.id = ?`, id))
	return g, wrapNotFound(err)
}

func (s *Store) RevokeGrant(ctx context.Context, id string, at time.Time) error {
	return s.exec(ctx, `UPDATE access_grants SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, toMs(at), id)
}

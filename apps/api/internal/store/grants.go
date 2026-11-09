package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const grantCols = `g.id, g.user_id, g.directory_id, g.video_id, g.qr_code_id, g.created_at, g.revoked_at`

func scanGrant(r scanner) (*model.AccessGrant, error) {
	var g model.AccessGrant
	var dir, vid, qr sql.NullString
	var revoked sql.NullInt64
	var created int64
	if err := r.Scan(&g.ID, &g.UserID, &dir, &vid, &qr, &created, &revoked); err != nil {
		return nil, err
	}
	g.DirectoryID = ptrStr(dir)
	g.VideoID = ptrStr(vid)
	g.QrCodeID = ptrStr(qr)
	g.CreatedAt = fromMs(created)
	g.RevokedAt = ptrTime(revoked)
	return &g, nil
}

// ActiveGrantRefs returns the directory ids and video ids the user holds active grants on,
// limited to items owned by creatorID.
func (s *Store) ActiveGrantRefs(ctx context.Context, userID, creatorID string) (dirIDs, videoIDs []string, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.directory_id, g.video_id
		 FROM access_grants g
		 LEFT JOIN directories d ON d.id = g.directory_id
		 LEFT JOIN videos v ON v.id = g.video_id
		 WHERE g.user_id = ? AND g.revoked_at IS NULL AND (d.owner_id = ? OR v.owner_id = ?)`,
		userID, creatorID, creatorID)
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
		`SELECT DISTINCT COALESCE(d.owner_id, v.owner_id)
		 FROM access_grants g
		 LEFT JOIN directories d ON d.id = g.directory_id
		 LEFT JOIN videos v ON v.id = g.video_id
		 WHERE g.user_id = ? AND g.revoked_at IS NULL AND COALESCE(d.owner_id, v.owner_id) IS NOT NULL`,
		userID)
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

func (s *Store) CreateGrant(ctx context.Context, userID string, directoryID, videoID, qrCodeID *string) (*model.AccessGrant, error) {
	g := &model.AccessGrant{ID: newID(), UserID: userID, DirectoryID: directoryID, VideoID: videoID, QrCodeID: qrCodeID, CreatedAt: now()}
	err := s.exec(ctx,
		`INSERT INTO access_grants (id, user_id, directory_id, video_id, qr_code_id, created_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		g.ID, g.UserID, nullStr(g.DirectoryID), nullStr(g.VideoID), nullStr(g.QrCodeID), toMs(g.CreatedAt))
	return g, err
}

// GrantWithUser is a grant plus the grantee's display name.
type GrantWithUser struct {
	Grant    model.AccessGrant
	UserName string
}

// ListActiveGrantsOn returns active grants placed directly on one directory or one video owned by ownerID.
func (s *Store) ListActiveGrantsOn(ctx context.Context, ownerID string, directoryID, videoID *string) ([]GrantWithUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+grantCols+`, u.name
		 FROM access_grants g
		 JOIN users u ON u.id = g.user_id
		 LEFT JOIN directories d ON d.id = g.directory_id
		 LEFT JOIN videos v ON v.id = g.video_id
		 WHERE g.revoked_at IS NULL AND g.directory_id IS ? AND g.video_id IS ?
		   AND COALESCE(d.owner_id, v.owner_id) = ?
		 ORDER BY g.created_at DESC`,
		nullStr(directoryID), nullStr(videoID), ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GrantWithUser{}
	for rows.Next() {
		var g model.AccessGrant
		var dir, vid, qr sql.NullString
		var revoked sql.NullInt64
		var created int64
		var name string
		if err := rows.Scan(&g.ID, &g.UserID, &dir, &vid, &qr, &created, &revoked, &name); err != nil {
			return nil, err
		}
		g.DirectoryID = ptrStr(dir)
		g.VideoID = ptrStr(vid)
		g.QrCodeID = ptrStr(qr)
		g.CreatedAt = fromMs(created)
		g.RevokedAt = ptrTime(revoked)
		out = append(out, GrantWithUser{Grant: g, UserName: name})
	}
	return out, rows.Err()
}

// GetGrantOwner returns a grant and the id of the creator who owns its target.
func (s *Store) GetGrantOwner(ctx context.Context, id string) (*model.AccessGrant, string, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+grantCols+`, COALESCE(d.owner_id, v.owner_id, '')
		 FROM access_grants g
		 LEFT JOIN directories d ON d.id = g.directory_id
		 LEFT JOIN videos v ON v.id = g.video_id
		 WHERE g.id = ?`, id)
	var g model.AccessGrant
	var dir, vid, qr sql.NullString
	var revoked sql.NullInt64
	var created int64
	var owner string
	if err := row.Scan(&g.ID, &g.UserID, &dir, &vid, &qr, &created, &revoked, &owner); err != nil {
		return nil, "", wrapNotFound(err)
	}
	g.DirectoryID = ptrStr(dir)
	g.VideoID = ptrStr(vid)
	g.QrCodeID = ptrStr(qr)
	g.CreatedAt = fromMs(created)
	g.RevokedAt = ptrTime(revoked)
	return &g, owner, nil
}

func (s *Store) RevokeGrant(ctx context.Context, id string, at time.Time) error {
	return s.exec(ctx, `UPDATE access_grants SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, toMs(at), id)
}

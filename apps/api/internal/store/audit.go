package store

import (
	"context"
	"database/sql"
	"time"
)

// AuditRow is one event derived from the grant and QR code tables. There is no audit table:
// grants and codes carry their own timestamps and IPs.
type AuditRow struct {
	At          time.Time
	Type        string // access.granted | access.revoked | qr.created | qr.revoked
	SourceID    string // grant id or QR code id
	SubjectID   *string
	IP          *string
	DirectoryID *string
	VideoID     *string
	QrCodeID    *string
}

const auditQuery = `
SELECT at, type, source_id, subject_id, ip, directory_id, video_id, qr_code_id FROM (
    SELECT g.created_at AS at, 'access.granted' AS type, g.id AS source_id, g.user_id AS subject_id,
           g.ip, g.directory_id, g.video_id, g.qr_code_id
      FROM access_grants g WHERE g.creator_id = ?1
    UNION ALL
    SELECT g.revoked_at, 'access.revoked', g.id, g.user_id, NULL, g.directory_id, g.video_id, g.qr_code_id
      FROM access_grants g WHERE g.creator_id = ?1 AND g.revoked_at IS NOT NULL
    UNION ALL
    SELECT q.created_at, 'qr.created', q.id, NULL, q.created_ip, q.directory_id, q.video_id, q.id
      FROM qr_codes q WHERE q.creator_id = ?1
    UNION ALL
    SELECT q.revoked_at, 'qr.revoked', q.id, NULL, NULL, q.directory_id, q.video_id, q.id
      FROM qr_codes q WHERE q.creator_id = ?1 AND q.revoked_at IS NOT NULL
)
WHERE ?2 IS NULL OR at < ?2
ORDER BY at DESC
LIMIT ?3`

// ListAudit returns a creator's events newest first, optionally strictly before a timestamp.
func (s *Store) ListAudit(ctx context.Context, creatorID string, limit int, before *time.Time) ([]AuditRow, error) {
	rows, err := s.db.QueryContext(ctx, auditQuery, creatorID, nullMs(before), limit)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r scanner) (*AuditRow, error) {
		var e AuditRow
		var at int64
		var subject, ip, dir, vid, qr sql.NullString
		if err := r.Scan(&at, &e.Type, &e.SourceID, &subject, &ip, &dir, &vid, &qr); err != nil {
			return nil, err
		}
		e.At = fromMs(at)
		e.SubjectID = ptrStr(subject)
		e.IP = ptrStr(ip)
		e.DirectoryID = ptrStr(dir)
		e.VideoID = ptrStr(vid)
		e.QrCodeID = ptrStr(qr)
		return &e, nil
	})
}

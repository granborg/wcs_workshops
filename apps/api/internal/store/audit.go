package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const auditCols = `id, at, type, creator_id, actor_id, subject_id, ip, directory_id, video_id, qr_code_id, detail`

func scanAudit(r scanner) (*model.AuditEvent, error) {
	var e model.AuditEvent
	var at int64
	var actor, subject, ip, dir, vid, qr, detail sql.NullString
	if err := r.Scan(&e.ID, &at, &e.Type, &e.CreatorID, &actor, &subject, &ip, &dir, &vid, &qr, &detail); err != nil {
		return nil, err
	}
	e.At = fromMs(at)
	e.ActorID = ptrStr(actor)
	e.SubjectID = ptrStr(subject)
	e.IP = ptrStr(ip)
	e.DirectoryID = ptrStr(dir)
	e.VideoID = ptrStr(vid)
	e.QrCodeID = ptrStr(qr)
	e.Detail = ptrStr(detail)
	return &e, nil
}

// InsertAudit stores an event. ID and At are filled in when zero.
func (s *Store) InsertAudit(ctx context.Context, e model.AuditEvent) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.At.IsZero() {
		e.At = now()
	}
	return s.exec(ctx, `INSERT INTO audit_events (`+auditCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, toMs(e.At), e.Type, e.CreatorID, nullStr(e.ActorID), nullStr(e.SubjectID), nullStr(e.IP),
		nullStr(e.DirectoryID), nullStr(e.VideoID), nullStr(e.QrCodeID), nullStr(e.Detail))
}

// ListAudit returns a creator's events newest first, optionally strictly before a timestamp.
func (s *Store) ListAudit(ctx context.Context, creatorID string, limit int, before *time.Time) ([]model.AuditEvent, error) {
	var rows *sql.Rows
	var err error
	if before != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+auditCols+` FROM audit_events WHERE creator_id = ? AND at < ? ORDER BY at DESC LIMIT ?`,
			creatorID, toMs(*before), limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+auditCols+` FROM audit_events WHERE creator_id = ? ORDER BY at DESC LIMIT ?`,
			creatorID, limit)
	}
	if err != nil {
		return nil, err
	}
	return collect(rows, scanAudit)
}

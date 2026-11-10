package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const dirCols = `id, owner_id, parent_id, name, created_at`

func scanDirectory(r scanner) (*model.Directory, error) {
	var d model.Directory
	var parent sql.NullString
	var created int64
	if err := r.Scan(&d.ID, &d.OwnerID, &parent, &d.Name, &created); err != nil {
		return nil, err
	}
	d.ParentID = ptrStr(parent)
	d.CreatedAt = fromMs(created)
	return &d, nil
}

// RootDirectory returns the creator's root, or nil when the user has no collection.
func (s *Store) RootDirectory(ctx context.Context, ownerID string) (*model.Directory, error) {
	d, err := scanDirectory(s.db.QueryRowContext(ctx,
		`SELECT `+dirCols+` FROM directories WHERE owner_id = ? AND parent_id IS NULL LIMIT 1`, ownerID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return d, err
}

func (s *Store) GetDirectory(ctx context.Context, id string) (*model.Directory, error) {
	d, err := scanDirectory(s.db.QueryRowContext(ctx, `SELECT `+dirCols+` FROM directories WHERE id = ?`, id))
	return d, wrapNotFound(err)
}

func (s *Store) ListDirectories(ctx context.Context, ownerID string) ([]model.Directory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+dirCols+` FROM directories WHERE owner_id = ? ORDER BY name COLLATE NOCASE`, ownerID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanDirectory)
}

func (s *Store) CreateDirectory(ctx context.Context, ownerID string, parentID *string, name string) (*model.Directory, error) {
	d := &model.Directory{ID: newID(), OwnerID: ownerID, ParentID: parentID, Name: name, CreatedAt: now()}
	err := s.exec(ctx, `INSERT INTO directories (`+dirCols+`) VALUES (?, ?, ?, ?, ?)`,
		d.ID, d.OwnerID, nullStr(d.ParentID), d.Name, toMs(d.CreatedAt))
	return d, err
}

// UpdateDirectory renames and/or re-parents. Nil fields are left alone.
func (s *Store) UpdateDirectory(ctx context.Context, id string, name, parentID *string) (*model.Directory, error) {
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if parentID != nil {
		sets = append(sets, "parent_id = ?")
		args = append(args, *parentID)
	}
	if len(sets) > 0 {
		args = append(args, id)
		if err := s.exec(ctx, `UPDATE directories SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
			return nil, err
		}
	}
	return s.GetDirectory(ctx, id)
}

// DeleteDirectory removes a directory. Sub-directories, QR codes and grants cascade.
// Videos inside must be moved or deleted first (the foreign key is RESTRICT).
func (s *Store) DeleteDirectory(ctx context.Context, id string) error {
	return s.exec(ctx, `DELETE FROM directories WHERE id = ?`, id)
}

// DeleteRootDirectories removes a creator's whole tree. The caller checks it holds no videos.
func (s *Store) DeleteRootDirectories(ctx context.Context, ownerID string) error {
	return s.exec(ctx, `DELETE FROM directories WHERE owner_id = ? AND parent_id IS NULL`, ownerID)
}

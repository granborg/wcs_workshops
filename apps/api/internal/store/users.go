package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const userCols = `id, auth_uid, email, name, role, created_at, updated_at`

func scanUser(r scanner) (*model.User, error) {
	var u model.User
	var email sql.NullString
	var created, updated int64
	if err := r.Scan(&u.ID, &u.AuthUID, &email, &u.Name, &u.Role, &created, &updated); err != nil {
		return nil, err
	}
	u.Email = ptrStr(email)
	u.CreatedAt = fromMs(created)
	u.UpdatedAt = fromMs(updated)
	return &u, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
	return u, wrapNotFound(err)
}

func (s *Store) GetUserByAuthUID(ctx context.Context, authUID string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE auth_uid = ?`, authUID))
	return u, wrapNotFound(err)
}

func (s *Store) CreateUser(ctx context.Context, authUID string, email *string, name string) (*model.User, error) {
	t := now()
	u := &model.User{ID: newID(), AuthUID: authUID, Email: email, Name: name, Role: model.RoleViewer, CreatedAt: t, UpdatedAt: t}
	err := s.exec(ctx, `INSERT INTO users (`+userCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.AuthUID, nullStr(u.Email), u.Name, u.Role, toMs(t), toMs(t))
	return u, err
}

// UpdateUser changes name and/or role. Nil fields are left alone.
func (s *Store) UpdateUser(ctx context.Context, id string, name, role *string) (*model.User, error) {
	sets := []string{"updated_at = ?"}
	args := []any{toMs(now())}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if role != nil {
		sets = append(sets, "role = ?")
		args = append(args, *role)
	}
	args = append(args, id)
	if err := s.exec(ctx, `UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		return nil, err
	}
	return s.GetUser(ctx, id)
}

// ListUsersByIDs returns the matching users ordered by name. Unknown ids are skipped.
func (s *Store) ListUsersByIDs(ctx context.Context, ids []string) ([]model.User, error) {
	if len(ids) == 0 {
		return []model.User{}, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userCols+` FROM users WHERE id IN (`+placeholders(len(ids))+`) ORDER BY name COLLATE NOCASE`,
		toAny(ids)...)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanUser)
}

package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const adminCols = `a.id, a.owner_id, a.directory_id, a.user_id, a.created_at`

func scanAdminInto(r scanner, a *model.CollectionAdmin, extra ...any) error {
	var created int64
	dest := append([]any{&a.ID, &a.OwnerID, &a.DirectoryID, &a.UserID, &created}, extra...)
	if err := r.Scan(dest...); err != nil {
		return err
	}
	a.CreatedAt = fromMs(created)
	return nil
}

// AdminDirectoryIDs returns the folders of one owner's collection where the user is an admin.
func (s *Store) AdminDirectoryIDs(ctx context.Context, userID, ownerID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT directory_id FROM collection_admins WHERE user_id = ? AND owner_id = ?`, userID, ownerID)
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

// OwnersAdministeredBy returns the ids of the collection owners who made the user an admin of anything.
func (s *Store) OwnersAdministeredBy(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT owner_id FROM collection_admins WHERE user_id = ?`, userID)
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

// AdminWithUser is an admin row plus the admin's name and email.
type AdminWithUser struct {
	Admin model.CollectionAdmin
	Name  string
	Email *string
}

// ListAdminsOn returns the admins placed directly on one folder, oldest first.
func (s *Store) ListAdminsOn(ctx context.Context, directoryID string) ([]AdminWithUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+adminCols+`, u.name, u.email
		 FROM collection_admins a JOIN users u ON u.id = a.user_id
		 WHERE a.directory_id = ? ORDER BY a.created_at`, directoryID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r scanner) (*AdminWithUser, error) {
		var aw AdminWithUser
		var email sql.NullString
		if err := scanAdminInto(r, &aw.Admin, &aw.Name, &email); err != nil {
			return nil, err
		}
		aw.Email = ptrStr(email)
		return &aw, nil
	})
}

func (s *Store) GetAdmin(ctx context.Context, id string) (*model.CollectionAdmin, error) {
	var a model.CollectionAdmin
	err := scanAdminInto(s.db.QueryRowContext(ctx, `SELECT `+adminCols+` FROM collection_admins a WHERE a.id = ?`, id), &a)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &a, nil
}

// AddAdmin makes the user an admin of the folder. Adding the same admin twice returns the existing row.
func (s *Store) AddAdmin(ctx context.Context, ownerID, directoryID, userID string) (*model.CollectionAdmin, error) {
	if err := s.exec(ctx,
		`INSERT INTO collection_admins (id, owner_id, directory_id, user_id, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (directory_id, user_id) DO NOTHING`,
		newID(), ownerID, directoryID, userID, toMs(now())); err != nil {
		return nil, err
	}
	var a model.CollectionAdmin
	err := scanAdminInto(s.db.QueryRowContext(ctx,
		`SELECT `+adminCols+` FROM collection_admins a WHERE a.directory_id = ? AND a.user_id = ?`, directoryID, userID), &a)
	return &a, err
}

func (s *Store) RemoveAdmin(ctx context.Context, id string) error {
	return s.exec(ctx, `DELETE FROM collection_admins WHERE id = ?`, id)
}

// GetCreatorByEmail finds a creator account by email, ignoring case. One person can have several accounts
// (one per sign-in provider) with the same email; the oldest creator account wins.
func (s *Store) GetCreatorByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE lower(email) = ? AND role = ? ORDER BY created_at LIMIT 1`,
		strings.ToLower(strings.TrimSpace(email)), model.RoleCreator))
	return u, wrapNotFound(err)
}

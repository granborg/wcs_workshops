package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// Collection admins are other creators who manage the videos, share codes and access of a folder and of every
// folder under it (see permissions.go). Only the collection owner adds and removes them.

// listAdmins returns the admins placed directly on one folder. Admins of a parent folder are listed on that folder.
func (s *Server) listAdmins(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	dir, err := s.ownedDirectory(r.Context(), u, r.URL.Query().Get("directoryId"))
	if err != nil {
		return err
	}
	admins, err := s.st.ListAdminsOn(r.Context(), dir.ID)
	if err != nil {
		return err
	}
	out := make([]model.AdminInfo, len(admins))
	for i, a := range admins {
		out[i] = adminInfo(a.Admin, a.Name, a.Email)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

type addAdminRequest struct {
	DirectoryID string `json:"directoryId"`
	Email       string `json:"email"`
}

// addAdmin makes the creator with this email an admin of the folder. Adding the same admin twice is not an error.
func (s *Server) addAdmin(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req addAdminRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	email := strings.TrimSpace(req.Email)
	if email == "" || len(email) > 320 {
		return httpx.BadRequest("Enter the email address of the person's account")
	}
	dir, err := s.ownedDirectory(ctx, u, req.DirectoryID)
	if err != nil {
		return err
	}
	admin, err := s.st.GetCreatorByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.BadRequest("No creator account uses this email. The person must sign in and choose Creator in Settings first.")
	}
	if err != nil {
		return err
	}
	if admin.ID == u.ID {
		return httpx.BadRequest("You own this collection already")
	}
	a, err := s.st.AddAdmin(ctx, u.ID, dir.ID, admin.ID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, adminInfo(*a, admin.Name, admin.Email))
	return nil
}

func (s *Server) removeAdmin(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !isUUID(id) {
		return httpx.NotFound("Admin")
	}
	a, err := s.st.GetAdmin(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && a.OwnerID != u.ID) {
		return httpx.NotFound("Admin")
	}
	if err != nil {
		return err
	}
	if err := s.st.RemoveAdmin(r.Context(), id); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

func adminInfo(a model.CollectionAdmin, name string, email *string) model.AdminInfo {
	return model.AdminInfo{
		ID:          a.ID,
		User:        model.UserRef{ID: a.UserID, Name: name},
		Email:       email,
		DirectoryID: a.DirectoryID,
		CreatedAt:   model.ISO(a.CreatedAt),
	}
}

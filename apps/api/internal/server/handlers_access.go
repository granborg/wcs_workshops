package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// listAccess returns active grants placed directly on a directory or video.
// Access inherited from a parent folder is listed on that folder.
func (s *Server) listAccess(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	dirID, vidID, err := queryTarget(r.URL.Query())
	if err != nil {
		return err
	}
	ownerID, err := s.managedTarget(r.Context(), u, dirID, vidID)
	if err != nil {
		return err
	}
	grants, err := s.st.ListActiveGrantsOn(r.Context(), ownerID, dirID, vidID)
	if err != nil {
		return err
	}
	out := make([]model.AccessGrantInfo, len(grants))
	for i, g := range grants {
		out[i] = model.AccessGrantInfo{
			ID:          g.Grant.ID,
			User:        model.UserRef{ID: g.Grant.UserID, Name: g.UserName},
			DirectoryID: g.Grant.DirectoryID,
			VideoID:     g.Grant.VideoID,
			QrCodeID:    g.Grant.QrCodeID,
			CreatedAt:   model.ISO(g.Grant.CreatedAt),
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// revokeAccess revokes one grant. The row is kept with revoked_at set, which is the audit record.
func (s *Server) revokeAccess(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !isUUID(id) {
		return httpx.NotFound("Access grant")
	}
	g, err := s.st.GetGrant(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.NotFound("Access grant")
	}
	if err != nil {
		return err
	}
	if _, err := s.managedTarget(r.Context(), u, g.DirectoryID, g.VideoID); err != nil {
		return httpx.NotFound("Access grant")
	}
	if err := s.st.RevokeGrant(r.Context(), id, time.Now()); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

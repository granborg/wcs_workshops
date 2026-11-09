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
	grants, err := s.st.ListActiveGrantsOn(r.Context(), u.ID, dirID, vidID)
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

// revokeAccess revokes one grant. The row is kept with revoked_at set, for the audit trail.
func (s *Server) revokeAccess(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	id := r.PathValue("id")
	if !isUUID(id) {
		return httpx.NotFound("Access grant")
	}
	g, owner, err := s.st.GetGrantOwner(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && owner != u.ID) {
		return httpx.NotFound("Access grant")
	}
	if err != nil {
		return err
	}
	if g.RevokedAt == nil {
		if err := s.st.RevokeGrant(ctx, id, time.Now()); err != nil {
			return err
		}
		s.audit(ctx, model.AuditEvent{
			Type: "access.revoked", CreatorID: u.ID, ActorID: &u.ID, SubjectID: &g.UserID, IP: clientIP(r),
			DirectoryID: g.DirectoryID, VideoID: g.VideoID, QrCodeID: g.QrCodeID,
		}, nil)
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

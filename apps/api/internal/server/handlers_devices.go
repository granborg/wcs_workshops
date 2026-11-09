package server

import (
	"net/http"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

type registerDeviceRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// registerDevice registers or re-assigns a push token to the signed-in user.
func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request, u *model.User) error {
	var req registerDeviceRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if len(req.Token) < 10 || len(req.Token) > 4096 {
		return httpx.BadRequest("token must be 10 to 4096 characters")
	}
	if req.Platform != "ios" && req.Platform != "android" && req.Platform != "web" {
		return httpx.BadRequest("platform must be ios, android, or web")
	}
	if err := s.st.UpsertDevice(r.Context(), u.ID, req.Token, req.Platform); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

// removeDevice forgets a push token, for sign-out.
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request, u *model.User) error {
	token := r.PathValue("token")
	if len(token) < 10 || len(token) > 4096 {
		return httpx.BadRequest("token must be 10 to 4096 characters")
	}
	if err := s.st.DeleteDevice(r.Context(), token, u.ID); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

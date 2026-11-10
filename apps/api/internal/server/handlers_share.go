package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// parseTarget picks exactly one of directoryId / videoId.
func parseTarget(directoryID, videoID *string) (*string, *string, error) {
	if directoryID != nil && *directoryID == "" {
		directoryID = nil
	}
	if videoID != nil && *videoID == "" {
		videoID = nil
	}
	if (directoryID == nil) == (videoID == nil) {
		return nil, nil, httpx.BadRequest("Provide exactly one of directoryId or videoId")
	}
	if directoryID != nil && !isUUID(*directoryID) {
		return nil, nil, httpx.NotFound("Directory")
	}
	if videoID != nil && !isUUID(*videoID) {
		return nil, nil, httpx.NotFound("Video")
	}
	return directoryID, videoID, nil
}

func queryTarget(q url.Values) (*string, *string, error) {
	d, v := q.Get("directoryId"), q.Get("videoId")
	return parseTarget(&d, &v)
}

func (s *Server) assertOwnedTarget(ctx context.Context, u *model.User, directoryID, videoID *string) error {
	if directoryID != nil {
		_, err := s.ownedDirectory(ctx, u, *directoryID)
		return err
	}
	_, err := s.ownedVideo(ctx, u, *videoID)
	return err
}

type createShareRequest struct {
	DirectoryID *string `json:"directoryId"`
	VideoID     *string `json:"videoId"`
	ExpiresAt   *string `json:"expiresAt"`
}

func randomCode() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// createShare creates a QR code for a directory or video, with an optional expiry.
func (s *Server) createShare(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req createShareRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	dirID, vidID, err := parseTarget(req.DirectoryID, req.VideoID)
	if err != nil {
		return err
	}
	if err := s.assertOwnedTarget(ctx, u, dirID, vidID); err != nil {
		return err
	}
	var expires *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return httpx.BadRequest("expiresAt must be an RFC 3339 timestamp")
		}
		if !t.After(time.Now()) {
			return httpx.BadRequest("Expiration must be in the future")
		}
		expires = &t
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	qr, err := s.st.CreateQrCode(ctx, code, u.ID, dirID, vidID, clientIP(r), expires)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, qr.ToInfo(s.publicURL(r)))
	return nil
}

// listShare returns QR codes for a directory or video, newest first.
func (s *Server) listShare(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	dirID, vidID, err := queryTarget(r.URL.Query())
	if err != nil {
		return err
	}
	if err := s.assertOwnedTarget(r.Context(), u, dirID, vidID); err != nil {
		return err
	}
	codes, err := s.st.ListQrCodes(r.Context(), u.ID, dirID, vidID)
	if err != nil {
		return err
	}
	base := s.publicURL(r)
	out := make([]model.QrCodeInfo, len(codes))
	for i, c := range codes {
		out[i] = c.ToInfo(base)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// revokeShare revokes a QR code. Grants already redeemed with it stay in place.
func (s *Server) revokeShare(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	id := r.PathValue("id")
	if !isUUID(id) {
		return httpx.NotFound("QR code")
	}
	qr, err := s.st.GetQrCode(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && qr.CreatorID != u.ID) {
		return httpx.NotFound("QR code")
	}
	if err != nil {
		return err
	}
	if qr.RevokedAt == nil {
		if err := s.st.RevokeQrCode(ctx, id, time.Now()); err != nil {
			return err
		}
		if qr, err = s.st.GetQrCode(ctx, id); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, qr.ToInfo(s.publicURL(r)))
	return nil
}

// publicURL is the origin that QR links point to. PUBLIC_URL wins when set. Otherwise the origin of this
// request is used: the Go binary and the Vite dev proxy both serve the web app on the host the caller used.
func (s *Server) publicURL(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

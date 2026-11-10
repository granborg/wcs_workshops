package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

type redeemRequest struct {
	Code string `json:"code"`
}

// extractCode accepts the bare code, the https://<host>/r/<code> link, the older wcsw://redeem/<code> payload,
// or any URL whose last segment is the code.
func extractCode(raw string) string {
	if strings.HasPrefix(raw, model.QRScheme) {
		return strings.TrimPrefix(raw, model.QRScheme)
	}
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil {
			parts := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}
	return raw
}

func (s *Server) redeem(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	var req redeemRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	raw := strings.TrimSpace(req.Code)
	if len(raw) < 8 || len(raw) > 200 {
		return httpx.BadRequest("code must be 8 to 200 characters")
	}

	qr, err := s.st.GetQrCodeByCode(ctx, extractCode(raw))
	if errors.Is(err, store.ErrNotFound) || (err == nil && qr.RevokedAt != nil) {
		return httpx.NotFound("QR code")
	}
	if err != nil {
		return err
	}
	if qr.ExpiresAt != nil && !qr.ExpiresAt.After(time.Now()) {
		return httpx.BadRequest("This QR code has expired")
	}
	if qr.CreatorID == u.ID {
		return httpx.BadRequest("This is your own collection")
	}

	creator, err := s.st.GetUser(ctx, qr.CreatorID)
	if err != nil {
		return err
	}
	root, err := s.st.RootDirectory(ctx, qr.CreatorID)
	if err != nil {
		return err
	}
	if root == nil {
		return httpx.NotFound("Collection")
	}

	existing, err := s.st.FindActiveGrant(ctx, u.ID, qr.DirectoryID, qr.VideoID)
	if err != nil {
		return err
	}
	if existing == nil {
		// The grant row is also the audit record: who, when, from which IP, with which code.
		if _, err := s.st.CreateGrant(ctx, u.ID, qr.CreatorID, qr.DirectoryID, qr.VideoID, &qr.ID, clientIP(r)); err != nil {
			return err
		}
	}

	httpx.WriteJSON(w, http.StatusOK, model.RedeemResult{
		Creator:     creatorSummary(creator, root.ID, u.ID, false),
		DirectoryID: qr.DirectoryID,
		VideoID:     qr.VideoID,
		Added:       existing == nil,
	})
	return nil
}

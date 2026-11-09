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

// extractCode accepts the bare code, the wcsw://redeem/<code> payload, or any URL whose last segment is the code.
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
	code := extractCode(raw)

	qr, err := s.st.GetQrCodeByCode(ctx, code)
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
		g, err := s.st.CreateGrant(ctx, u.ID, qr.DirectoryID, qr.VideoID, &qr.ID)
		if err != nil {
			return err
		}
		s.audit(ctx, model.AuditEvent{
			Type: "access.granted", CreatorID: qr.CreatorID, ActorID: &u.ID, SubjectID: &u.ID, IP: clientIP(r),
			DirectoryID: qr.DirectoryID, VideoID: qr.VideoID, QrCodeID: &qr.ID,
		}, map[string]any{"grantId": g.ID})

		what := "a video"
		data := map[string]string{"creatorId": qr.CreatorID}
		if qr.DirectoryID != nil {
			what = "a folder"
			data["directoryId"] = *qr.DirectoryID
		} else if qr.VideoID != nil {
			data["videoId"] = *qr.VideoID
		}
		go func(userID, body string) {
			bg, cancel := background()
			defer cancel()
			s.notifier.NotifyUser(bg, userID, "New access", body, data)
		}(u.ID, creator.Name+" shared "+what+" with you")
	}

	httpx.WriteJSON(w, http.StatusOK, model.RedeemResult{
		Creator:     s.creatorSummary(creator, root.ID, u.ID),
		DirectoryID: qr.DirectoryID,
		VideoID:     qr.VideoID,
		Added:       existing == nil,
	})
	return nil
}

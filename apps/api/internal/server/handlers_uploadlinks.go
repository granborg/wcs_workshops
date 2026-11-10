package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// An upload link lets any signed-in user, viewers included, upload the file of one video that has no file yet:
// for example a dancer who filmed an instructor's workshop. The owner or an admin of the video creates the link.
// The link stops working when the video has a file, or when it is revoked. When the upload finishes, the uploader
// gets access to watch that video (see uploadLinkComplete).

type createUploadLinkRequest struct {
	VideoID string `json:"videoId"`
}

func (s *Server) createUploadLink(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req createUploadLinkRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	v, err := s.managedVideo(ctx, u, req.VideoID)
	if err != nil {
		return err
	}
	if v.HasFile() {
		return httpx.BadRequest("This video already has a file")
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	l, err := s.st.CreateUploadLink(ctx, code, v.OwnerID, v.ID, clientIP(r))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, l.ToInfo(s.publicURL(r)))
	return nil
}

// listUploadLinks returns the links of one video, newest first.
func (s *Server) listUploadLinks(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	v, err := s.managedVideo(r.Context(), u, r.URL.Query().Get("videoId"))
	if err != nil {
		return err
	}
	links, err := s.st.ListUploadLinks(r.Context(), v.ID)
	if err != nil {
		return err
	}
	base := s.publicURL(r)
	out := make([]model.UploadLinkInfo, len(links))
	for i, l := range links {
		out[i] = l.ToInfo(base)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) revokeUploadLink(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	id := r.PathValue("id")
	if !isUUID(id) {
		return httpx.NotFound("Upload link")
	}
	l, err := s.st.GetUploadLink(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.NotFound("Upload link")
	}
	if err != nil {
		return err
	}
	if _, err := s.managedVideo(ctx, u, l.VideoID); err != nil {
		return httpx.NotFound("Upload link")
	}
	if l.RevokedAt == nil {
		if err := s.st.RevokeUploadLink(ctx, id, time.Now()); err != nil {
			return err
		}
		if l, err = s.st.GetUploadLink(ctx, id); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, l.ToInfo(s.publicURL(r)))
	return nil
}

// openUploadLink finds an active link by its code, with its video. A revoked link answers 404, as a missing one.
func (s *Server) openUploadLink(ctx context.Context, code string) (*model.UploadLink, *model.Video, error) {
	if len(code) < 8 || len(code) > 200 {
		return nil, nil, httpx.NotFound("Upload link")
	}
	l, err := s.st.GetUploadLinkByCode(ctx, code)
	if errors.Is(err, store.ErrNotFound) || (err == nil && l.RevokedAt != nil) {
		return nil, nil, httpx.NotFound("Upload link")
	}
	if err != nil {
		return nil, nil, err
	}
	v, err := s.st.GetVideo(ctx, l.VideoID)
	if err != nil {
		return nil, nil, err
	}
	return l, v, nil
}

func (s *Server) uploadLinkTarget(ctx context.Context, v *model.Video) (*model.UploadLinkTarget, error) {
	owner, err := s.st.GetUser(ctx, v.OwnerID)
	if err != nil {
		return nil, err
	}
	return &model.UploadLinkTarget{VideoID: v.ID, VideoTitle: v.Title, CreatorName: owner.Name, Open: !v.HasFile()}, nil
}

// getUploadLinkTarget tells the person who opened the link what they upload to, and whether the link still works.
func (s *Server) getUploadLinkTarget(w http.ResponseWriter, r *http.Request, u *model.User) error {
	_, v, err := s.openUploadLink(r.Context(), r.PathValue("code"))
	if err != nil {
		return err
	}
	t, err := s.uploadLinkTarget(r.Context(), v)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, t)
	return nil
}

func (s *Server) uploadLinkTicket(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	var req videoUploadTicketRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.SizeBytes <= 0 {
		return httpx.BadRequest("sizeBytes must be positive")
	}
	l, v, err := s.openUploadLink(ctx, r.PathValue("code"))
	if err != nil {
		return err
	}
	ticket, err := s.startUpload(ctx, v, req.SizeBytes)
	if err != nil {
		return err
	}
	// The link row records who uploaded, from where, and when. The audit log shows it.
	if err := s.st.MarkUploadLinkUsed(ctx, l.ID, u.ID, clientIP(r)); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, ticket)
	return nil
}

// uploadLinkComplete is step 2: the device finished sending the file. Only the user who started the upload may call it.
func (s *Server) uploadLinkComplete(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	l, v, err := s.openUploadLink(ctx, r.PathValue("code"))
	if err != nil {
		return err
	}
	if l.UsedBy == nil || *l.UsedBy != u.ID {
		return httpx.BadRequest("Start the upload with this link first")
	}
	if v, err = s.markUploaded(ctx, v); err != nil {
		return err
	}
	// The uploader may watch what they uploaded. The grant is on the video only, so it does not open the folder,
	// and the owner can revoke it like any other access. A grant with no QR code means "access by uploading".
	if v.OwnerID != u.ID {
		existing, err := s.st.FindActiveGrant(ctx, u.ID, nil, &v.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			if _, err := s.st.CreateGrant(ctx, u.ID, v.OwnerID, nil, &v.ID, nil, clientIP(r)); err != nil {
				return err
			}
		}
	}
	t, err := s.uploadLinkTarget(ctx, v)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, t)
	return nil
}

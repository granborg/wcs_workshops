package server

import (
	"context"
	"errors"
	"math"
	"net/http"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
)

type uploadTicketRequest struct {
	Title       string `json:"title"`
	DirectoryID string `json:"directoryId"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// uploadTicket is step 1 of upload: reserve the video on Vimeo and return the tus link for the device.
func (s *Server) uploadTicket(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req uploadTicketRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	title, err := validName(&req.Title, 120)
	if err != nil {
		return err
	}
	if req.SizeBytes <= 0 {
		return httpx.BadRequest("sizeBytes must be positive")
	}
	dir, err := s.ownedDirectory(ctx, u, req.DirectoryID)
	if err != nil {
		return err
	}
	created, err := s.vimeo.CreateTusUpload(ctx, *title, req.SizeBytes)
	if err != nil {
		return vimeoErr(err)
	}
	v, err := s.st.CreateVideo(ctx, u.ID, dir.ID, *title, &created.VimeoID, created.Hash)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, model.UploadTicket{VideoID: v.ID, UploadLink: created.UploadLink})
	return nil
}

// refreshStatus asks Vimeo for the transcode state and stores it. Vimeo errors leave the row unchanged.
func (s *Server) refreshStatus(ctx context.Context, v *model.Video) (*model.Video, error) {
	if v.VimeoID == nil || v.Status == model.StatusReady || v.Status == model.StatusFailed {
		return v, nil
	}
	remote, err := s.vimeo.GetVideo(ctx, *v.VimeoID)
	if err != nil {
		if !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo status check failed", "video", v.ID, "err", err)
		}
		return v, nil
	}
	status := v.Status
	switch {
	case remote.Transcode != nil && remote.Transcode.Status == "complete":
		status = model.StatusReady
	case remote.Transcode != nil && remote.Transcode.Status == "error",
		remote.Upload != nil && remote.Upload.Status == "error":
		status = model.StatusFailed
	case remote.Upload != nil && remote.Upload.Status == "complete":
		status = model.StatusProcessing
	}
	var dur *int
	if remote.Duration > 0 {
		d := int(math.Round(remote.Duration))
		dur = &d
	}
	return s.st.UpdateVideoStatus(ctx, v.ID, status, dur)
}

// completeVideo is step 2 of upload: the device finished sending bytes.
func (s *Server) completeVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	v, err := s.ownedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.Status == model.StatusUploading {
		if v, err = s.st.UpdateVideoStatus(ctx, v.ID, model.StatusProcessing, nil); err != nil {
			return err
		}
	}
	if v, err = s.refreshStatus(ctx, v); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, v.ToSummary())
	return nil
}

func (s *Server) visibleVideo(ctx context.Context, u *model.User, id string) (*model.Video, error) {
	if !isUUID(id) {
		return nil, httpx.NotFound("Video")
	}
	v, err := s.st.GetVideo(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.NotFound("Video")
	}
	if err != nil {
		return nil, err
	}
	ok, err := s.canView(ctx, u.ID, v)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("Video")
	}
	return v, nil
}

func (s *Server) getVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	v, err := s.visibleVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.Status == model.StatusProcessing {
		if v, err = s.refreshStatus(ctx, v); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, v.ToSummary())
	return nil
}

func (s *Server) playback(w http.ResponseWriter, r *http.Request, u *model.User) error {
	v, err := s.visibleVideo(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.VimeoID == nil {
		return httpx.BadRequest("Video has not been uploaded yet")
	}
	httpx.WriteJSON(w, http.StatusOK, model.PlaybackInfo{
		VideoID: v.ID, VimeoID: *v.VimeoID, Hash: v.VimeoHash, EmbedURL: vimeo.EmbedURL(*v.VimeoID, v.VimeoHash),
	})
	return nil
}

type patchVideoRequest struct {
	Title       *string `json:"title"`
	DirectoryID *string `json:"directoryId"`
}

// patchVideo renames and/or moves. Direct grants follow the video; inherited access changes implicitly.
func (s *Server) patchVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req patchVideoRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.Title == nil && req.DirectoryID == nil {
		return httpx.BadRequest("Nothing to change")
	}
	title, err := validName(req.Title, 120)
	if err != nil {
		return err
	}
	v, err := s.ownedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if req.DirectoryID != nil {
		if _, err := s.ownedDirectory(ctx, u, *req.DirectoryID); err != nil {
			return err
		}
	}
	updated, err := s.st.UpdateVideo(ctx, v.ID, title, req.DirectoryID)
	if err != nil {
		return err
	}
	if title != nil && v.VimeoID != nil {
		if err := s.vimeo.Rename(ctx, *v.VimeoID, *title); err != nil && !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo rename failed", "video", v.ID, "err", err)
		}
	}
	if req.DirectoryID != nil && *req.DirectoryID != v.DirectoryID {
		s.audit(ctx, model.AuditEvent{Type: "video.moved", CreatorID: u.ID, ActorID: &u.ID, IP: clientIP(r), VideoID: &v.ID},
			map[string]any{"from": v.DirectoryID, "to": *req.DirectoryID})
	}
	httpx.WriteJSON(w, http.StatusOK, updated.ToSummary())
	return nil
}

func (s *Server) deleteVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	v, err := s.ownedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.VimeoID != nil {
		if err := s.vimeo.Delete(ctx, *v.VimeoID); err != nil && !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo delete failed; removing local record anyway", "video", v.ID, "err", err)
		}
	}
	if err := s.st.DeleteVideo(ctx, v.ID); err != nil {
		return err
	}
	s.audit(ctx, model.AuditEvent{Type: "video.deleted", CreatorID: u.ID, ActorID: &u.ID, IP: clientIP(r), VideoID: &v.ID}, nil)
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

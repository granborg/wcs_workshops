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
	dir, err := s.managedDirectory(ctx, u, req.DirectoryID)
	if err != nil {
		return err
	}
	created, err := s.vimeo.CreateTusUpload(ctx, *title, req.SizeBytes)
	if err != nil {
		return vimeoErr(err)
	}
	// The video belongs to the collection owner, also when an admin uploads it.
	v, err := s.st.CreateVideo(ctx, dir.OwnerID, dir.ID, *title, &created.VimeoID, created.Hash)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, model.UploadTicket{VideoID: v.ID, UploadLink: created.UploadLink})
	return nil
}

type placeholderRequest struct {
	Title       string `json:"title"`
	DirectoryID string `json:"directoryId"`
}

// createPlaceholder adds a video with no file yet. It shows in the library and can be shared now;
// the creator uploads the file later through videoUploadTicket.
func (s *Server) createPlaceholder(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req placeholderRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	title, err := validName(&req.Title, 120)
	if err != nil {
		return err
	}
	dir, err := s.managedDirectory(ctx, u, req.DirectoryID)
	if err != nil {
		return err
	}
	v, err := s.st.CreateVideo(ctx, dir.OwnerID, dir.ID, *title, nil, nil)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, v.ToSummary())
	return nil
}

type videoUploadTicketRequest struct {
	SizeBytes int64 `json:"sizeBytes"`
}

// videoUploadTicket starts an upload for an existing video: a placeholder, an upload that did not finish,
// or a video that failed on Vimeo. Grants and share codes stay on the video, so viewers get the file
// when it is ready. A previous Vimeo video, if any, is deleted after the new one is reserved.
func (s *Server) videoUploadTicket(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req videoUploadTicketRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.SizeBytes <= 0 {
		return httpx.BadRequest("sizeBytes must be positive")
	}
	v, err := s.managedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	ticket, err := s.startUpload(ctx, v, req.SizeBytes)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, ticket)
	return nil
}

// startUpload reserves a new Vimeo upload for a video that has no file yet, and points the video at it.
// A previous Vimeo video, if any, is deleted after the new one is reserved.
func (s *Server) startUpload(ctx context.Context, v *model.Video, sizeBytes int64) (*model.UploadTicket, error) {
	if v.HasFile() {
		return nil, httpx.BadRequest("This video already has a file")
	}
	created, err := s.vimeo.CreateTusUpload(ctx, v.Title, sizeBytes)
	if err != nil {
		return nil, vimeoErr(err)
	}
	if _, err := s.st.AttachUpload(ctx, v.ID, created.VimeoID, created.Hash); err != nil {
		return nil, err
	}
	if v.VimeoID != nil {
		if err := s.vimeo.Delete(ctx, *v.VimeoID); err != nil && !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo delete of replaced upload failed", "video", v.ID, "err", err)
		}
	}
	return &model.UploadTicket{VideoID: v.ID, UploadLink: created.UploadLink}, nil
}

// needsRefresh is true while Vimeo may still have news: the video is uploading or processing, or it is ready
// but Vimeo had not generated its thumbnail yet when the status was last checked.
func needsRefresh(v *model.Video) bool {
	if v.VimeoID == nil || v.Status == model.StatusFailed || v.Status == model.StatusPlaceholder {
		return false
	}
	return v.Status != model.StatusReady || v.ThumbnailURL == nil
}

// refreshStatus asks Vimeo for the transcode state, frame size and thumbnail and stores them.
// Vimeo errors leave the row unchanged.
func (s *Server) refreshStatus(ctx context.Context, v *model.Video) (*model.Video, error) {
	if !needsRefresh(v) {
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
	var meta store.VideoMeta
	if remote.Duration > 0 {
		d := int(math.Round(remote.Duration))
		meta.DurationSec = &d
	}
	if remote.Width > 0 && remote.Height > 0 {
		meta.Width, meta.Height = &remote.Width, &remote.Height
	}
	// Before the transcode is complete, Vimeo's picture is a placeholder. Do not keep that one.
	if status == model.StatusReady {
		meta.ThumbnailURL = remote.Thumbnail()
	}
	return s.st.UpdateVideoStatus(ctx, v.ID, status, meta)
}

// completeVideo is step 2 of upload: the device finished sending bytes.
func (s *Server) completeVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	v, err := s.managedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v, err = s.markUploaded(ctx, v); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, v.ToSummary())
	return nil
}

// markUploaded records that the device finished sending the file, then asks Vimeo for the status.
func (s *Server) markUploaded(ctx context.Context, v *model.Video) (*model.Video, error) {
	if v.Status == model.StatusUploading {
		var err error
		if v, err = s.st.UpdateVideoStatus(ctx, v.ID, model.StatusProcessing, store.VideoMeta{}); err != nil {
			return nil, err
		}
	}
	return s.refreshStatus(ctx, v)
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
	ok, err := s.canView(ctx, u, v)
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
	// Uploading videos are refreshed by the completion call, not here: the device may still be sending bytes.
	if v.Status == model.StatusProcessing || v.Status == model.StatusReady {
		if v, err = s.refreshStatus(ctx, v); err != nil {
			return err
		}
	}
	out := v.ToSummary()
	if out.CanManage, err = s.managesDir(ctx, u, v.OwnerID, v.DirectoryID); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
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
	v, err := s.managedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if req.DirectoryID != nil {
		// Videos move only within their own collection.
		target, err := s.managedDirectory(ctx, u, *req.DirectoryID)
		if err != nil {
			return err
		}
		if target.OwnerID != v.OwnerID {
			return httpx.NotFound("Directory")
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
	httpx.WriteJSON(w, http.StatusOK, updated.ToSummary())
	return nil
}

// deleteVideoEverywhere removes the video from Vimeo (best effort) and from the database.
func (s *Server) deleteVideoEverywhere(ctx context.Context, v *model.Video) error {
	if v.VimeoID != nil {
		if err := s.vimeo.Delete(ctx, *v.VimeoID); err != nil && !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo delete failed; removing local record anyway", "video", v.ID, "err", err)
		}
	}
	return s.st.DeleteVideo(ctx, v.ID)
}

func (s *Server) deleteVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	v, err := s.managedVideo(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if err := s.deleteVideoEverywhere(r.Context(), v); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
)

type uploadTicketRequest struct {
	Title       string  `json:"title"`
	DirectoryID string  `json:"directoryId"`
	VideoDate   *string `json:"videoDate"`
	SizeBytes   int64   `json:"sizeBytes"`
}

// validVideoDate accepts nil, or a calendar date as YYYY-MM-DD. With allowEmpty, "" means "clear the date".
func validVideoDate(d *string, allowEmpty bool) error {
	if d == nil || (allowEmpty && *d == "") {
		return nil
	}
	if _, err := time.Parse(time.DateOnly, *d); err != nil {
		return httpx.BadRequest("videoDate must be a date as YYYY-MM-DD")
	}
	return nil
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
	if err := validVideoDate(req.VideoDate, false); err != nil {
		return err
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
	v, err := s.st.CreateVideo(ctx, dir.OwnerID, dir.ID, *title, req.VideoDate, &created.VimeoID, created.Hash)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, model.UploadTicket{VideoID: v.ID, UploadLink: created.UploadLink})
	return nil
}

type placeholderRequest struct {
	Title       string  `json:"title"`
	DirectoryID string  `json:"directoryId"`
	VideoDate   *string `json:"videoDate"`
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
	if err := validVideoDate(req.VideoDate, false); err != nil {
		return err
	}
	dir, err := s.managedDirectory(ctx, u, req.DirectoryID)
	if err != nil {
		return err
	}
	v, err := s.st.CreateVideo(ctx, dir.OwnerID, dir.ID, *title, req.VideoDate, nil, nil)
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
	ticket, _, err := s.startUpload(ctx, v, req.SizeBytes)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, ticket)
	return nil
}

// startUpload reserves a new Vimeo upload for a video that has no file yet, and points the video at it.
// A previous Vimeo video, if any, is deleted after the new one is reserved. It returns the updated video.
func (s *Server) startUpload(ctx context.Context, v *model.Video, sizeBytes int64) (*model.UploadTicket, *model.Video, error) {
	if v.HasFile() {
		return nil, nil, httpx.BadRequest("This video already has a file")
	}
	created, err := s.vimeo.CreateTusUpload(ctx, v.Title, sizeBytes)
	if err != nil {
		return nil, nil, vimeoErr(err)
	}
	updated, err := s.st.AttachUpload(ctx, v.ID, created.VimeoID, created.Hash)
	if err != nil {
		return nil, nil, err
	}
	if v.VimeoID != nil {
		if err := s.vimeo.Delete(ctx, *v.VimeoID); err != nil && !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo delete of replaced upload failed", "video", v.ID, "err", err)
		}
	}
	return &model.UploadTicket{VideoID: v.ID, UploadLink: created.UploadLink}, updated, nil
}

// mediaRecheck is how long the app waits before it asks Vimeo again for a missing size or thumbnail.
// Vimeo can make the thumbnail a short time after processing is done.
const mediaRecheck = 10 * time.Minute

// wantsRefresh reports whether a page that shows the video should ask Vimeo about it first: it is still
// processing, or it is ready but its size or thumbnail is missing. An upload that is in progress (or was
// abandoned) is not asked about here; the uploader's device reports when it is done.
func wantsRefresh(v *model.Video) bool {
	if v.VimeoID == nil {
		return false
	}
	if v.Status == model.StatusProcessing {
		return true
	}
	return v.Status == model.StatusReady && missingMedia(v) &&
		(v.MediaCheckedAt == nil || time.Since(*v.MediaCheckedAt) > mediaRecheck)
}

func missingMedia(v *model.Video) bool {
	return v.Width == nil || v.Height == nil || v.ThumbnailURL == nil
}

// refreshStatus asks Vimeo for the transcode state, size and thumbnail, and stores them. Vimeo errors leave
// the row unchanged.
func (s *Server) refreshStatus(ctx context.Context, v *model.Video) (*model.Video, error) {
	if v.VimeoID == nil || v.Status == model.StatusFailed || (v.Status == model.StatusReady && !missingMedia(v)) {
		return v, nil
	}
	remote, err := s.vimeo.GetVideo(ctx, *v.VimeoID)
	if err != nil {
		if !errors.Is(err, vimeo.ErrNotConfigured) {
			s.log.Warn("vimeo status check failed", "video", v.ID, "err", err)
		}
		return v, nil
	}
	return s.storeRemote(ctx, v, remote)
}

// storeRemote stores the status, size and thumbnail that Vimeo reported for the video.
func (s *Server) storeRemote(ctx context.Context, v *model.Video, remote *vimeo.Video) (*model.Video, error) {
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
	var dur, width, height *int
	if remote.Duration > 0 {
		d := int(math.Round(remote.Duration))
		dur = &d
	}
	if remote.Width > 0 && remote.Height > 0 {
		width, height = &remote.Width, &remote.Height
	}
	var thumb *string
	if t := remote.Thumbnail(); t != "" {
		thumb = &t
	}
	return s.st.UpdateVideoFromVimeo(ctx, v.ID, status, dur, width, height, thumb)
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

// errUploadIncomplete answers a "complete" call while Vimeo does not have the whole file. The app tries again,
// because Vimeo can take a moment to report the end of an upload.
var errUploadIncomplete = httpx.NewError(http.StatusConflict, "upload_incomplete",
	"Vimeo has not received the whole file. Keep the app open until the upload finishes.")

// markUploaded records that the file arrived, after Vimeo confirms it. The device's word is not enough:
// a video with a file closes its upload links and stops new uploads, and an upload link gives access.
// A retry after success finds the video already processing or ready, and only refreshes it.
func (s *Server) markUploaded(ctx context.Context, v *model.Video) (*model.Video, error) {
	if v.HasFile() {
		return s.refreshStatus(ctx, v)
	}
	if v.Status != model.StatusUploading || v.VimeoID == nil {
		return nil, httpx.BadRequest("There is no upload in progress for this video")
	}
	remote, err := s.vimeo.GetVideo(ctx, *v.VimeoID)
	if err != nil {
		return nil, vimeoErr(err)
	}
	switch {
	case remote.Upload != nil && remote.Upload.Status == "complete":
		return s.storeRemote(ctx, v, remote) // processing, or ready when Vimeo is already done
	case remote.Upload != nil && remote.Upload.Status == "error":
		if _, err := s.storeRemote(ctx, v, remote); err != nil { // failed: the owner can upload again
			return nil, err
		}
		return nil, httpx.Conflict("Vimeo could not receive the file. Upload it again.")
	default:
		return nil, errUploadIncomplete
	}
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
	if wantsRefresh(v) {
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

// videoDownloads lists the sizes of the video file that Vimeo can give, largest first, with temporary links.
// Only the owner and admins can download, as on vimeo.com, where downloads are turned off for viewers.
func (s *Server) videoDownloads(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	v, err := s.managedVideo(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if v.VimeoID == nil || v.Status != model.StatusReady {
		return httpx.BadRequest("The video is not ready to download")
	}
	files, err := s.vimeo.Downloads(ctx, *v.VimeoID)
	if err != nil {
		return vimeoErr(err)
	}
	out := make([]model.VideoDownload, 0, len(files))
	for _, f := range files {
		if f.Link == "" {
			continue
		}
		label := f.Rendition
		if f.Quality == "source" || f.Rendition == "source" {
			label = "Original"
		} else if label == "" {
			label = f.PublicName
		}
		var expires *string
		if f.Expires != "" {
			expires = &f.Expires
		}
		out = append(out, model.VideoDownload{
			Label: label, Width: f.Width, Height: f.Height, SizeBytes: f.Size, URL: f.Link, ExpiresAt: expires,
		})
	}
	// The original first, then the largest size first.
	slices.SortStableFunc(out, func(a, b model.VideoDownload) int {
		if (a.Label == "Original") != (b.Label == "Original") {
			if a.Label == "Original" {
				return -1
			}
			return 1
		}
		return b.Height - a.Height
	})
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

type patchVideoRequest struct {
	Title       *string `json:"title"`
	DirectoryID *string `json:"directoryId"`
	// VideoDate is YYYY-MM-DD, or "" to clear the date.
	VideoDate *string `json:"videoDate"`
}

// patchVideo renames, moves and/or changes the date. Direct grants follow the video; inherited access changes
// implicitly.
func (s *Server) patchVideo(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req patchVideoRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.Title == nil && req.DirectoryID == nil && req.VideoDate == nil {
		return httpx.BadRequest("Nothing to change")
	}
	title, err := validName(req.Title, 120)
	if err != nil {
		return err
	}
	if err := validVideoDate(req.VideoDate, true); err != nil {
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
	updated, err := s.st.UpdateVideo(ctx, v.ID, title, req.DirectoryID, req.VideoDate)
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

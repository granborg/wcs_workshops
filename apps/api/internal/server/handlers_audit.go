package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

// listAudit returns the creator's access history, derived from grants and QR codes.
func (s *Server) listAudit(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	q := r.URL.Query()

	limit := 50
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			return httpx.BadRequest("limit must be between 1 and 200")
		}
		limit = n
	}
	var before *time.Time
	if raw := q.Get("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return httpx.BadRequest("before must be an RFC 3339 timestamp")
		}
		before = &t
	}

	events, err := s.st.ListAudit(ctx, u.ID, limit, before)
	if err != nil {
		return err
	}

	idSet := map[string]bool{}
	for _, e := range events {
		if e.SubjectID != nil {
			idSet[*e.SubjectID] = true
		}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	users, err := s.st.ListUsersByIDs(ctx, ids)
	if err != nil {
		return err
	}
	names := make(map[string]string, len(users))
	for _, usr := range users {
		names[usr.ID] = usr.Name
	}

	// Every event belongs to this creator, so their own folders and videos name all targets.
	// Deleting a folder or video deletes its codes and grants, so a missing target is unusual.
	targets, err := s.auditTargetNames(r, u)
	if err != nil {
		return err
	}

	out := make([]model.AuditEventInfo, len(events))
	for i, e := range events {
		info := model.AuditEventInfo{
			ID: e.Type + ":" + e.SourceID, At: model.ISO(e.At), Type: e.Type, IP: e.IP,
			DirectoryID: e.DirectoryID, VideoID: e.VideoID, QrCodeID: e.QrCodeID,
		}
		switch {
		case e.VideoID != nil:
			info.TargetName = nameOr(targets[*e.VideoID], "Deleted video")
		case e.DirectoryID != nil:
			info.TargetName = nameOr(targets[*e.DirectoryID], "Deleted folder")
		}
		if e.SubjectID != nil {
			name, ok := names[*e.SubjectID]
			if !ok {
				name = "Deleted user"
			}
			info.Subject = &model.UserRef{ID: *e.SubjectID, Name: name}
		}
		out[i] = info
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// auditTargetNames maps the creator's folder and video ids to the names the app shows for them.
func (s *Server) auditTargetNames(r *http.Request, u *model.User) (map[string]string, error) {
	ctx := r.Context()
	dirs, err := s.st.ListDirectories(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	videos, err := s.st.ListVideos(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(dirs)+len(videos))
	for _, d := range dirs {
		out[d.ID] = d.Name
		if d.ParentID == nil {
			out[d.ID] = u.Name
		}
	}
	for _, v := range videos {
		out[v.ID] = v.Title
	}
	return out, nil
}

func nameOr(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}

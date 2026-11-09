package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

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
		if e.ActorID != nil {
			idSet[*e.ActorID] = true
		}
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
	ref := func(id *string) *model.UserRef {
		if id == nil {
			return nil
		}
		name, ok := names[*id]
		if !ok {
			name = "Deleted user"
		}
		return &model.UserRef{ID: *id, Name: name}
	}

	out := make([]model.AuditEventInfo, len(events))
	for i, e := range events {
		info := model.AuditEventInfo{
			ID: e.ID, At: model.ISO(e.At), Type: e.Type,
			Actor: ref(e.ActorID), Subject: ref(e.SubjectID), IP: e.IP,
			DirectoryID: e.DirectoryID, VideoID: e.VideoID, QrCodeID: e.QrCodeID,
		}
		if e.Detail != nil {
			_ = json.Unmarshal([]byte(*e.Detail), &info.Detail)
		}
		out[i] = info
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

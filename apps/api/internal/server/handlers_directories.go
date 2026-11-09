package server

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

func validName(s *string, max int) (*string, error) {
	if s == nil {
		return nil, nil
	}
	n := strings.TrimSpace(*s)
	if n == "" || utf8.RuneCountInString(n) > max {
		return nil, httpx.BadRequest("name must be 1 to " + itoa(max) + " characters")
	}
	return &n, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type createDirectoryRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
}

func (s *Server) createDirectory(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	var req createDirectoryRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	name, err := validName(&req.Name, 120)
	if err != nil {
		return err
	}
	parent, err := s.ownedDirectory(r.Context(), u, req.ParentID)
	if err != nil {
		return err
	}
	d, err := s.st.CreateDirectory(r.Context(), u.ID, &parent.ID, *name)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, d.ToRecord())
	return nil
}

type patchDirectoryRequest struct {
	Name     *string `json:"name"`
	ParentID *string `json:"parentId"`
}

// patchDirectory renames and/or moves. Renaming never touches grants. Moving changes inherited access implicitly.
func (s *Server) patchDirectory(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	var req patchDirectoryRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.Name == nil && req.ParentID == nil {
		return httpx.BadRequest("Nothing to change")
	}
	name, err := validName(req.Name, 120)
	if err != nil {
		return err
	}
	dir, err := s.ownedDirectory(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}

	if req.ParentID != nil {
		if dir.ParentID == nil {
			return httpx.BadRequest("The root directory cannot be moved")
		}
		if *req.ParentID == dir.ID {
			return httpx.BadRequest("A directory cannot be moved into itself")
		}
		if _, err := s.ownedDirectory(ctx, u, *req.ParentID); err != nil {
			return err
		}
		col, err := access.Load(ctx, s.st, u.ID)
		if err != nil {
			return err
		}
		if col != nil && col.Contains(dir.ID, *req.ParentID) {
			return httpx.BadRequest("A directory cannot be moved into one of its own sub-directories")
		}
	}

	updated, err := s.st.UpdateDirectory(ctx, dir.ID, name, req.ParentID)
	if err != nil {
		return err
	}
	if req.ParentID != nil && *req.ParentID != *dir.ParentID {
		s.audit(ctx, model.AuditEvent{
			Type: "directory.moved", CreatorID: u.ID, ActorID: &u.ID, IP: clientIP(r), DirectoryID: &dir.ID,
		}, map[string]any{"from": *dir.ParentID, "to": *req.ParentID})
	}
	httpx.WriteJSON(w, http.StatusOK, updated.ToRecord())
	return nil
}

// deleteDirectory removes a directory and its sub-directories.
// ?mode=delete removes every video inside (also on Vimeo). ?mode=move&targetDirectoryId=... relocates them first.
func (s *Server) deleteDirectory(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	ctx := r.Context()
	mode := r.URL.Query().Get("mode")
	target := r.URL.Query().Get("targetDirectoryId")
	if mode != "delete" && mode != "move" {
		return httpx.BadRequest("mode must be delete or move")
	}
	dir, err := s.ownedDirectory(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if dir.ParentID == nil {
		return httpx.BadRequest("The root directory cannot be deleted")
	}

	col, err := access.Load(ctx, s.st, u.ID)
	if err != nil {
		return err
	}
	if col == nil {
		return httpx.NotFound("Collection")
	}
	subtree := map[string]bool{}
	for _, id := range col.SubtreeIDs(dir.ID) {
		subtree[id] = true
	}
	videos := make([]model.Video, 0)
	for _, v := range col.Videos {
		if subtree[v.DirectoryID] {
			videos = append(videos, v)
		}
	}

	if mode == "move" {
		if subtree[target] {
			return httpx.BadRequest("Target directory is inside the directory being deleted")
		}
		if _, err := s.ownedDirectory(ctx, u, target); err != nil {
			return err
		}
		ids := make([]string, len(videos))
		for i, v := range videos {
			ids[i] = v.ID
		}
		if err := s.st.MoveVideos(ctx, ids, target); err != nil {
			return err
		}
	} else {
		for _, v := range videos {
			if v.VimeoID != nil {
				if err := s.vimeo.Delete(ctx, *v.VimeoID); err != nil {
					s.log.Warn("vimeo delete failed; removing local record anyway", "video", v.ID, "err", err)
				}
			}
			if err := s.st.DeleteVideo(ctx, v.ID); err != nil {
				return err
			}
			vid := v.ID
			s.audit(ctx, model.AuditEvent{Type: "video.deleted", CreatorID: u.ID, ActorID: &u.ID, IP: clientIP(r), VideoID: &vid}, nil)
		}
	}

	if err := s.st.DeleteDirectory(ctx, dir.ID); err != nil {
		return err
	}
	s.audit(ctx, model.AuditEvent{
		Type: "directory.deleted", CreatorID: u.ID, ActorID: &u.ID, IP: clientIP(r), DirectoryID: &dir.ID,
	}, map[string]any{"mode": mode, "videos": len(videos)})
	httpx.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

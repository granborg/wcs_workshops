package server

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

func (s *Server) toMe(ctx context.Context, u *model.User) (*model.Me, error) {
	me := &model.Me{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role}
	if u.Role == model.RoleCreator {
		root, err := s.st.RootDirectory(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		if root != nil {
			me.RootDirectoryID = &root.ID
		}
	}
	return me, nil
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request, u *model.User) error {
	me, err := s.toMe(r.Context(), u)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, me)
	return nil
}

type patchMeRequest struct {
	Name *string `json:"name"`
	Role *string `json:"role"`
}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	var req patchMeRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || utf8.RuneCountInString(n) > 80 {
			return httpx.BadRequest("name must be 1 to 80 characters")
		}
		req.Name = &n
	}
	if req.Role != nil && *req.Role != model.RoleViewer && *req.Role != model.RoleCreator {
		return httpx.BadRequest("role must be viewer or creator")
	}

	if req.Role != nil && *req.Role != u.Role {
		if *req.Role == model.RoleCreator {
			root, err := s.st.RootDirectory(ctx, u.ID)
			if err != nil {
				return err
			}
			if root == nil {
				if _, err := s.st.CreateDirectory(ctx, u.ID, nil, "Collection"); err != nil {
					return err
				}
			}
		} else {
			n, err := s.st.CountVideos(ctx, u.ID)
			if err != nil {
				return err
			}
			if n > 0 {
				return httpx.Conflict("Delete every video in your collection before switching to a viewer account")
			}
			// Deleting the root cascades to sub-directories, QR codes and grants.
			if err := s.st.DeleteRootDirectories(ctx, u.ID); err != nil {
				return err
			}
		}
	}

	updated, err := s.st.UpdateUser(ctx, u.ID, req.Name, req.Role)
	if err != nil {
		return err
	}
	me, err := s.toMe(ctx, updated)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, me)
	return nil
}

package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

const maxBannerBytes = 5 << 20

var bannerExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

func (s *Server) toMe(ctx context.Context, u *model.User) (*model.Me, error) {
	me := &model.Me{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, BannerURL: s.bannerURL(u.BannerURL)}
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

func (s *Server) removeBannerFile(path *string) {
	if path == nil || !strings.HasPrefix(*path, "/static/") {
		return
	}
	_ = os.Remove(filepath.Join(s.cfg.UploadDir, filepath.Base(*path)))
}

func (s *Server) putBanner(w http.ResponseWriter, r *http.Request, u *model.User) error {
	if err := requireCreator(u); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBannerBytes+64<<10)
	if err := r.ParseMultipartForm(maxBannerBytes); err != nil {
		return httpx.BadRequest("Banner must be an image of 5 MB or smaller")
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return httpx.BadRequest("No file uploaded")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	ext, ok := bannerExt[http.DetectContentType(data)]
	if !ok {
		return httpx.BadRequest("Banner must be PNG, JPEG, or WebP")
	}
	name := u.ID + "-" + uuid.NewString() + ext
	if err := os.WriteFile(filepath.Join(s.cfg.UploadDir, name), data, 0o644); err != nil {
		return err
	}
	s.removeBannerFile(u.BannerURL)
	url := "/static/" + name
	updated, err := s.st.SetBanner(r.Context(), u.ID, &url)
	if err != nil {
		return err
	}
	me, err := s.toMe(r.Context(), updated)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, me)
	return nil
}

func (s *Server) deleteBanner(w http.ResponseWriter, r *http.Request, u *model.User) error {
	s.removeBannerFile(u.BannerURL)
	updated, err := s.st.SetBanner(r.Context(), u.ID, nil)
	if err != nil {
		return err
	}
	me, err := s.toMe(r.Context(), updated)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, me)
	return nil
}

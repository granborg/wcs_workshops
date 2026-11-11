package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// listLibrary returns my own collection first (when I am a creator), then the collections of creators who
// shared with me or made me an admin.
func (s *Server) listLibrary(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	out := make([]model.CreatorSummary, 0)

	if u.Role == model.RoleCreator {
		root, err := s.st.RootDirectory(ctx, u.ID)
		if err != nil {
			return err
		}
		if root != nil {
			out = append(out, creatorSummary(u, root.ID, u.ID, false))
		}
	}

	ids, err := s.st.CreatorsSharedWith(ctx, u.ID)
	if err != nil {
		return err
	}
	adminOf := map[string]bool{}
	if u.Role == model.RoleCreator {
		owners, err := s.st.OwnersAdministeredBy(ctx, u.ID)
		if err != nil {
			return err
		}
		for _, id := range owners {
			adminOf[id] = true
			ids = append(ids, id)
		}
	}
	creators, err := s.st.ListUsersByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range creators {
		if creators[i].ID == u.ID {
			continue
		}
		root, err := s.st.RootDirectory(ctx, creators[i].ID)
		if err != nil {
			return err
		}
		if root != nil {
			out = append(out, creatorSummary(&creators[i], root.ID, u.ID, adminOf[creators[i].ID]))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// getLibraryTree returns one creator's tree pruned to what I may see. Owners get the full tree.
func (s *Server) getLibraryTree(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	creatorID := r.PathValue("creatorId")
	if !isUUID(creatorID) {
		return httpx.NotFound("Creator")
	}
	creator, err := s.st.GetUser(ctx, creatorID)
	if errors.Is(err, store.ErrNotFound) {
		return httpx.NotFound("Creator")
	}
	if err != nil {
		return err
	}
	col, err := access.Load(ctx, s.st, creatorID)
	if err != nil {
		return err
	}
	if col == nil {
		return httpx.NotFound("Creator")
	}
	s.refreshFromVimeo(ctx, col.Videos)

	own := creator.ID == u.ID
	var grants *access.Grants
	adminDirs := []string{}
	if !own {
		// Admins see everything under their folders, as if those folders were shared with them.
		grants, adminDirs, err = s.viewerGrants(ctx, u, creatorID)
		if err != nil {
			return err
		}
		if grants.Empty() {
			return httpx.Forbidden("No access to this collection")
		}
	}

	httpx.WriteJSON(w, http.StatusOK, model.LibraryTree{
		Creator:           creatorSummary(creator, col.Root.ID, u.ID, len(adminDirs) > 0),
		CanManage:         own,
		AdminDirectoryIDs: adminDirs,
		Root:              col.BuildTree(grants),
	})
	return nil
}

// refreshFromVimeo asks Vimeo about each video that is still processing, or that has no size or thumbnail yet
// (see wantsRefresh), and updates videos in place. Without this, the folder list shows "Processing" until
// someone opens the video. A slow or failing Vimeo leaves the stored values; a later load tries again.
func (s *Server) refreshFromVimeo(ctx context.Context, videos []model.Video) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4) // at most 4 Vimeo requests at a time
	for i := range videos {
		v := &videos[i]
		if !wantsRefresh(v) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			updated, err := s.refreshStatus(ctx, v)
			if err != nil {
				s.log.Warn("refresh video status failed", "video", v.ID, "err", err)
				return
			}
			*v = *updated
		}()
	}
	wg.Wait()
}

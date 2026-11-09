package server

import (
	"errors"
	"net/http"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// listLibrary returns creators who shared with me. My own collection comes first when I am a creator.
func (s *Server) listLibrary(w http.ResponseWriter, r *http.Request, u *model.User) error {
	ctx := r.Context()
	out := make([]model.CreatorSummary, 0)

	if u.Role == model.RoleCreator {
		root, err := s.st.RootDirectory(ctx, u.ID)
		if err != nil {
			return err
		}
		if root != nil {
			out = append(out, s.creatorSummary(u, root.ID, u.ID))
		}
	}

	ids, err := s.st.CreatorsSharedWith(ctx, u.ID)
	if err != nil {
		return err
	}
	others := ids[:0]
	for _, id := range ids {
		if id != u.ID {
			others = append(others, id)
		}
	}
	creators, err := s.st.ListUsersByIDs(ctx, others)
	if err != nil {
		return err
	}
	for i := range creators {
		root, err := s.st.RootDirectory(ctx, creators[i].ID)
		if err != nil {
			return err
		}
		if root != nil {
			out = append(out, s.creatorSummary(&creators[i], root.ID, u.ID))
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

	own := creator.ID == u.ID
	var grants *access.Grants
	if !own {
		dirs, vids, err := s.st.ActiveGrantRefs(ctx, u.ID, creatorID)
		if err != nil {
			return err
		}
		grants = access.NewGrants(dirs, vids)
		if grants.Empty() {
			return httpx.Forbidden("No access to this collection")
		}
	}

	httpx.WriteJSON(w, http.StatusOK, model.LibraryTree{
		Creator:   s.creatorSummary(creator, col.Root.ID, u.ID),
		CanManage: own,
		Root:      col.BuildTree(grants),
	})
	return nil
}

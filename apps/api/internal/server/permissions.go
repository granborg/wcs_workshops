package server

import (
	"context"
	"errors"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// Managing means: add, rename, move and remove videos, make placeholders and upload links, and manage share codes
// and access. The owner manages the whole collection. A collection admin manages the folders they were made admin
// of, and every folder under them. Only the owner manages folders and admins (see ownedDirectory).

// managesDir reports whether u may manage the folder dirID of ownerID's collection.
func (s *Server) managesDir(ctx context.Context, u *model.User, ownerID, dirID string) (bool, error) {
	if ownerID == u.ID {
		return true, nil
	}
	if u.Role != model.RoleCreator {
		return false, nil
	}
	adminDirs, err := s.st.AdminDirectoryIDs(ctx, u.ID, ownerID)
	if err != nil || len(adminDirs) == 0 {
		return false, err
	}
	col, err := access.Load(ctx, s.st, ownerID)
	if err != nil || col == nil {
		return false, err
	}
	admin := make(map[string]bool, len(adminDirs))
	for _, id := range adminDirs {
		admin[id] = true
	}
	for _, id := range col.AncestorIDs(dirID) {
		if admin[id] {
			return true, nil
		}
	}
	return false, nil
}

// managedDirectory returns the folder when u may manage it. Otherwise it answers 404, as for a missing folder.
func (s *Server) managedDirectory(ctx context.Context, u *model.User, id string) (*model.Directory, error) {
	if !isUUID(id) {
		return nil, httpx.NotFound("Directory")
	}
	d, err := s.st.GetDirectory(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.NotFound("Directory")
	}
	if err != nil {
		return nil, err
	}
	ok, err := s.managesDir(ctx, u, d.OwnerID, d.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("Directory")
	}
	return d, nil
}

// managedVideo returns the video when u may manage it. Otherwise it answers 404, as for a missing video.
func (s *Server) managedVideo(ctx context.Context, u *model.User, id string) (*model.Video, error) {
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
	ok, err := s.managesDir(ctx, u, v.OwnerID, v.DirectoryID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("Video")
	}
	return v, nil
}

// managedTarget checks that u may manage a folder or a video, and returns the collection owner's id.
func (s *Server) managedTarget(ctx context.Context, u *model.User, directoryID, videoID *string) (string, error) {
	if directoryID != nil {
		d, err := s.managedDirectory(ctx, u, *directoryID)
		if err != nil {
			return "", err
		}
		return d.OwnerID, nil
	}
	v, err := s.managedVideo(ctx, u, *videoID)
	if err != nil {
		return "", err
	}
	return v.OwnerID, nil
}

// viewerGrants is what u may see in ownerID's collection: folders and videos granted to u, plus the folders
// where u is an admin. An admin sees everything under those folders.
func (s *Server) viewerGrants(ctx context.Context, u *model.User, ownerID string) (*access.Grants, []string, error) {
	dirs, vids, err := s.st.ActiveGrantRefs(ctx, u.ID, ownerID)
	if err != nil {
		return nil, nil, err
	}
	adminDirs := []string{}
	if u.Role == model.RoleCreator {
		if adminDirs, err = s.st.AdminDirectoryIDs(ctx, u.ID, ownerID); err != nil {
			return nil, nil, err
		}
	}
	return access.NewGrants(append(dirs, adminDirs...), vids), adminDirs, nil
}

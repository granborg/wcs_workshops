// Package access resolves who may see what. See docs/ARCHITECTURE.md, "Access model".
package access

import (
	"context"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// Collection is a creator's full directory tree and video list, loaded in two queries.
type Collection struct {
	Dirs     []model.Directory
	DirsByID map[string]model.Directory
	Videos   []model.Video
	Root     model.Directory
}

// Load reads a creator's collection. It returns nil when the user has no root directory.
func Load(ctx context.Context, st *store.Store, creatorID string) (*Collection, error) {
	dirs, err := st.ListDirectories(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	videos, err := st.ListVideos(ctx, creatorID)
	if err != nil {
		return nil, err
	}
	col := &Collection{Dirs: dirs, DirsByID: make(map[string]model.Directory, len(dirs)), Videos: videos}
	found := false
	for _, d := range dirs {
		col.DirsByID[d.ID] = d
		if d.ParentID == nil && !found {
			col.Root = d
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	return col, nil
}

// AncestorIDs lists the directory and every ancestor up to the root.
func (c *Collection) AncestorIDs(dirID string) []string {
	out := []string{}
	cur, ok := c.DirsByID[dirID]
	for ok {
		out = append(out, cur.ID)
		if cur.ParentID == nil {
			break
		}
		cur, ok = c.DirsByID[*cur.ParentID]
	}
	return out
}

// SubtreeIDs lists the directory and every descendant.
func (c *Collection) SubtreeIDs(dirID string) []string {
	children := c.childrenIndex()
	out := []string{dirID}
	for i := 0; i < len(out); i++ {
		for _, child := range children[out[i]] {
			out = append(out, child.ID)
		}
	}
	return out
}

// Contains reports whether id is in the subtree rooted at dirID.
func (c *Collection) Contains(dirID, id string) bool {
	for _, x := range c.SubtreeIDs(dirID) {
		if x == id {
			return true
		}
	}
	return false
}

func (c *Collection) childrenIndex() map[string][]model.Directory {
	idx := make(map[string][]model.Directory)
	for _, d := range c.Dirs {
		if d.ParentID != nil {
			idx[*d.ParentID] = append(idx[*d.ParentID], d)
		}
	}
	return idx
}

// Grants is the set of items a viewer holds active grants on.
type Grants struct {
	Dirs   map[string]bool
	Videos map[string]bool
}

// NewGrants builds a Grants set from id lists.
func NewGrants(dirIDs, videoIDs []string) *Grants {
	g := &Grants{Dirs: make(map[string]bool, len(dirIDs)), Videos: make(map[string]bool, len(videoIDs))}
	for _, id := range dirIDs {
		g.Dirs[id] = true
	}
	for _, id := range videoIDs {
		g.Videos[id] = true
	}
	return g
}

// Empty reports whether the viewer holds no grants at all.
func (g *Grants) Empty() bool { return len(g.Dirs) == 0 && len(g.Videos) == 0 }

// VideoVisible reports whether a video is visible: granted directly, or under a granted directory.
func (c *Collection) VideoVisible(v model.Video, g *Grants) bool {
	if g.Videos[v.ID] {
		return true
	}
	for _, id := range c.AncestorIDs(v.DirectoryID) {
		if g.Dirs[id] {
			return true
		}
	}
	return false
}

// BuildTree renders the collection as nested nodes. With g == nil the full tree is returned (owner view).
// Otherwise the tree is pruned: a directory is kept when it is granted, lies under a granted directory,
// or contains something visible further down.
func (c *Collection) BuildTree(g *Grants) *model.DirectoryNode {
	children := c.childrenIndex()
	videosOf := make(map[string][]model.Video)
	for _, v := range c.Videos {
		if g != nil && !c.VideoVisible(v, g) {
			continue
		}
		videosOf[v.DirectoryID] = append(videosOf[v.DirectoryID], v)
	}

	var build func(d model.Directory, inherited bool) *model.DirectoryNode
	build = func(d model.Directory, inherited bool) *model.DirectoryNode {
		granted := inherited || g == nil || g.Dirs[d.ID]
		node := &model.DirectoryNode{
			ID:       d.ID,
			Name:     d.Name,
			ParentID: d.ParentID,
			Children: make([]*model.DirectoryNode, 0),
			Videos:   make([]model.VideoSummary, 0),
		}
		for _, child := range children[d.ID] {
			if n := build(child, granted); n != nil {
				node.Children = append(node.Children, n)
			}
		}
		for _, v := range videosOf[d.ID] {
			node.Videos = append(node.Videos, v.ToSummary())
		}
		if !granted && len(node.Children) == 0 && len(node.Videos) == 0 {
			return nil
		}
		return node
	}

	if n := build(c.Root, false); n != nil {
		return n
	}
	return &model.DirectoryNode{
		ID: c.Root.ID, Name: c.Root.Name, ParentID: nil,
		Children: make([]*model.DirectoryNode, 0), Videos: make([]model.VideoSummary, 0),
	}
}

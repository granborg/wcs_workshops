package server

import (
	"testing"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

// The app asks Vimeo about a video only while it processes, or while its size or thumbnail is missing, and
// then at most every mediaRecheck.
func TestWantsRefresh(t *testing.T) {
	id, n, thumb := "123", 640, "https://i.vimeocdn.com/x.jpg"
	recent, old := time.Now().Add(-time.Minute), time.Now().Add(-2*mediaRecheck)
	complete := model.Video{VimeoID: &id, Status: model.StatusReady, Width: &n, Height: &n, ThumbnailURL: &thumb}
	noThumb := complete
	noThumb.ThumbnailURL = nil

	for _, tc := range []struct {
		name string
		v    model.Video
		want bool
	}{
		{"placeholder", model.Video{Status: model.StatusPlaceholder}, false},
		{"uploading", model.Video{VimeoID: &id, Status: model.StatusUploading}, false},
		{"processing", model.Video{VimeoID: &id, Status: model.StatusProcessing}, true},
		{"failed", model.Video{VimeoID: &id, Status: model.StatusFailed}, false},
		{"ready and complete", complete, false},
		{"ready, never checked", model.Video{VimeoID: &id, Status: model.StatusReady}, true},
		{"ready, no thumbnail, checked recently", func() model.Video { v := noThumb; v.MediaCheckedAt = &recent; return v }(), false},
		{"ready, no thumbnail, checked long ago", func() model.Video { v := noThumb; v.MediaCheckedAt = &old; return v }(), true},
	} {
		if got := wantsRefresh(&tc.v); got != tc.want {
			t.Errorf("%s: wantsRefresh = %v, want %v", tc.name, got, tc.want)
		}
	}
}

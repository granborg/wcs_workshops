package main

import (
	"context"
	"fmt"
	"os"

	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
)

// runVimeoDownloads lists the download sizes Vimeo gives for one video, to check that the account's plan allows
// downloads through the API. It changes nothing and does not print the links. It returns the process exit code.
func runVimeoDownloads(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: api vimeo-downloads <vimeo id>")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		return 1
	}
	client := vimeo.New(cfg.VimeoAccessToken, cfg.VimeoPrivacy, cfg.VimeoEmbedDomains)
	files, err := client.Downloads(context.Background(), args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read downloads:", err)
		return 1
	}
	withLink := 0
	for _, f := range files {
		hasLink := f.Link != ""
		if hasLink {
			withLink++
		}
		fmt.Printf("quality=%s rendition=%s %dx%d size=%d link=%t expires=%s\n",
			f.Quality, f.Rendition, f.Width, f.Height, f.Size, hasLink, f.Expires)
	}
	fmt.Printf("\n%d sizes, %d with a download link.\n", len(files), withLink)
	return 0
}

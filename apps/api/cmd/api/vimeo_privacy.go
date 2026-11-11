package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/db"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
)

const vimeoPrivacyUsage = `Usage:
  api vimeo-privacy check              Show the Vimeo plan and the privacy of every video. Changes nothing.
  api vimeo-privacy apply -video <id>  Apply the configured privacy to one video (a Vimeo id), then read it back.
  api vimeo-privacy apply -all         Apply the configured privacy to every video, then read each one back.

The configured privacy is VIMEO_PRIVACY and VIMEO_EMBED_DOMAINS, the same settings new uploads get.
To try other settings first, add -privacy <unlisted|disable> and -domains <a.com,b.com>. These change only
this run, not the settings for new uploads.
On Fly.io: fly ssh console -C "/api vimeo-privacy check"
`

type videoRef struct {
	vimeoID string
	title   string
}

// runVimeoPrivacy is a one-time maintenance command: it gives videos that were uploaded with older settings the
// current privacy settings. It returns the process exit code.
func runVimeoPrivacy(args []string) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "apply") {
		fmt.Fprint(os.Stderr, vimeoPrivacyUsage)
		return 2
	}
	mode := args[0]
	fs := flag.NewFlagSet("vimeo-privacy "+mode, flag.ContinueOnError)
	one := fs.String("video", "", "apply to this Vimeo video id only")
	all := fs.Bool("all", false, "apply to every video in the database")
	privacy := fs.String("privacy", "", "for this run only: use this instead of VIMEO_PRIVACY (unlisted or disable)")
	domains := fs.String("domains", "", "for this run only: use these comma-separated domains instead of VIMEO_EMBED_DOMAINS")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if mode == "apply" && (*one == "") == !*all {
		fmt.Fprintln(os.Stderr, "apply needs exactly one of -video <id> or -all")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		return 1
	}
	if *privacy != "" {
		cfg.VimeoPrivacy = *privacy
	}
	if *domains != "" {
		cfg.VimeoEmbedDomains = strings.Split(*domains, ",")
	}
	if cfg.VimeoPrivacy != "unlisted" && cfg.VimeoPrivacy != "disable" {
		fmt.Fprintln(os.Stderr, "privacy must be unlisted or disable")
		return 2
	}
	if cfg.VimeoPrivacy == "disable" && len(cfg.VimeoEmbedDomains) == 0 {
		fmt.Fprintln(os.Stderr, "disable needs at least one embed domain: a video with none plays nowhere")
		return 2
	}
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open database:", err)
		return 1
	}
	defer database.Close()

	ctx := context.Background()
	client := vimeo.New(cfg.VimeoAPIURL, cfg.VimeoAccessToken, cfg.VimeoPrivacy, cfg.VimeoEmbedDomains)
	name, account, err := client.Account(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read Vimeo account:", err)
		return 1
	}
	fmt.Printf("Vimeo account: %s (plan: %s)\n", name, account)
	fmt.Printf("Target:        privacy=%s embed domains=%s\n\n", cfg.VimeoPrivacy, strings.Join(cfg.VimeoEmbedDomains, ","))

	videos, err := listVimeoVideos(ctx, database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list videos:", err)
		return 1
	}
	if *one != "" {
		videos = filterVideo(videos, *one)
		if len(videos) == 0 {
			fmt.Fprintf(os.Stderr, "no video with Vimeo id %s in the database\n", *one)
			return 1
		}
	}

	failed := 0
	for _, v := range videos {
		if mode == "apply" {
			if err := client.ApplyPrivacy(ctx, v.vimeoID); err != nil {
				fmt.Printf("FAILED     %s  %q: %v\n", v.vimeoID, v.title, err)
				failed++
				continue
			}
		}
		// Read back what Vimeo stored: it can accept a request and keep a different value.
		p, err := client.GetPrivacy(ctx, v.vimeoID)
		if err != nil {
			fmt.Printf("READ ERROR %s  %q: %v\n", v.vimeoID, v.title, err)
			failed++
			continue
		}
		status := "OK        "
		if !client.Wants(p) {
			status = "DIFFERENT "
			failed++
		}
		fmt.Printf("%s %s  %q: view=%s embed=%s domains=%s\n",
			status, v.vimeoID, v.title, p.View, p.Embed, strings.Join(p.Domains, ","))
	}

	fmt.Printf("\n%d videos, %d match the target privacy, %d do not.\n", len(videos), len(videos)-failed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// listVimeoVideos returns the videos that have a file on Vimeo. Placeholders have no Vimeo id yet.
func listVimeoVideos(ctx context.Context, database *sql.DB) ([]videoRef, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT vimeo_id, title FROM videos WHERE vimeo_id IS NOT NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []videoRef{}
	for rows.Next() {
		var v videoRef
		if err := rows.Scan(&v.vimeoID, &v.title); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func filterVideo(videos []videoRef, vimeoID string) []videoRef {
	for _, v := range videos {
		if v.vimeoID == vimeoID {
			return []videoRef{v}
		}
	}
	return nil
}

// Package config reads the API configuration from environment variables and an optional .env file.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every runtime setting. Defaults suit local development.
type Config struct {
	Addr         string
	DatabasePath string
	CORSOrigins  []string
	PublicURL    string // origin QR links point to, e.g. https://wcs-workshops.fly.dev; empty = request origin
	// BehindFlyProxy is true on Fly.io (FLY_APP_NAME is set there). Only then does the server trust the
	// Fly-Client-IP header, which Fly Proxy sets; elsewhere any client could send it.
	BehindFlyProxy    bool
	FirebaseProjectID string
	VimeoAPIURL       string // Vimeo's API; only tests change it
	VimeoAccessToken  string
	VimeoPrivacy      string // "unlisted" or "disable"
	VimeoEmbedDomains []string
}

// Load reads .env from the working directory when present, then the environment.
//
// Two Firebase variables are read by the Firebase Admin SDK itself, not here:
// GOOGLE_APPLICATION_CREDENTIALS (service account, production) and
// FIREBASE_AUTH_EMULATOR_HOST (local Auth emulator, development).
func Load() (Config, error) {
	_ = godotenv.Load()

	c := Config{
		Addr:              get("HOST", "0.0.0.0") + ":" + get("PORT", "3000"),
		DatabasePath:      get("DATABASE_PATH", "./data/wcs.db"),
		CORSOrigins:       list("CORS_ORIGINS", "http://localhost:5173,capacitor://localhost,http://localhost"),
		PublicURL:         strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"),
		BehindFlyProxy:    os.Getenv("FLY_APP_NAME") != "",
		FirebaseProjectID: os.Getenv("FIREBASE_PROJECT_ID"),
		VimeoAPIURL:       "https://api.vimeo.com",
		VimeoAccessToken:  os.Getenv("VIMEO_ACCESS_TOKEN"),
		VimeoPrivacy:      get("VIMEO_PRIVACY", "unlisted"),
		VimeoEmbedDomains: list("VIMEO_EMBED_DOMAINS", ""),
	}

	if c.VimeoPrivacy != "unlisted" && c.VimeoPrivacy != "disable" {
		return c, fmt.Errorf("VIMEO_PRIVACY must be unlisted or disable, got %q", c.VimeoPrivacy)
	}
	// With no allowed domain, a "disable" video plays nowhere, not even in this app.
	if c.VimeoPrivacy == "disable" && len(c.VimeoEmbedDomains) == 0 {
		return c, fmt.Errorf("VIMEO_PRIVACY=disable needs at least one domain in VIMEO_EMBED_DOMAINS")
	}
	return c, nil
}

func get(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func list(name, fallback string) []string {
	raw := get(name, fallback)
	out := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

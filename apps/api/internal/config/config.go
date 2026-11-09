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
	Addr              string
	DatabasePath      string
	PublicAPIURL      string
	CORSOrigins       []string
	DevAuthBypass     bool
	VimeoAccessToken  string
	VimeoPrivacy      string // "unlisted" or "disable"
	VimeoEmbedDomains []string
	UploadDir         string
	Production        bool
}

// Load reads .env from the working directory when present, then the environment.
func Load() (Config, error) {
	_ = godotenv.Load()

	c := Config{
		Addr:              get("HOST", "0.0.0.0") + ":" + get("PORT", "3000"),
		DatabasePath:      get("DATABASE_PATH", "./data/wcs.db"),
		PublicAPIURL:      strings.TrimRight(get("PUBLIC_API_URL", "http://localhost:3000"), "/"),
		CORSOrigins:       list("CORS_ORIGINS", "http://localhost:5173,capacitor://localhost,http://localhost"),
		DevAuthBypass:     boolean("DEV_AUTH_BYPASS", false),
		VimeoAccessToken:  os.Getenv("VIMEO_ACCESS_TOKEN"),
		VimeoPrivacy:      get("VIMEO_PRIVACY", "unlisted"),
		VimeoEmbedDomains: list("VIMEO_EMBED_DOMAINS", ""),
		UploadDir:         get("UPLOAD_DIR", "./uploads"),
		Production:        os.Getenv("APP_ENV") == "production",
	}

	if c.VimeoPrivacy != "unlisted" && c.VimeoPrivacy != "disable" {
		return c, fmt.Errorf("VIMEO_PRIVACY must be unlisted or disable, got %q", c.VimeoPrivacy)
	}
	if c.Production && c.DevAuthBypass {
		return c, fmt.Errorf("DEV_AUTH_BYPASS must not be enabled in production")
	}
	return c, nil
}

func get(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func boolean(name string, fallback bool) bool {
	switch strings.ToLower(os.Getenv(name)) {
	case "":
		return fallback
	case "1", "true", "yes":
		return true
	default:
		return false
	}
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

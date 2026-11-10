// Package server wires routes, authentication and handlers.
package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/auth"
	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
	"github.com/granborg/wcs_workshops/apps/api/web"
)

// Server holds the dependencies handlers need.
type Server struct {
	cfg      config.Config
	st       *store.Store
	vimeo    *vimeo.Client
	verifier auth.Verifier
	log      *slog.Logger
	mux      *http.ServeMux
}

// New builds a Server.
func New(cfg config.Config, database *sql.DB, verifier auth.Verifier, log *slog.Logger) *Server {
	s := &Server{
		cfg:      cfg,
		st:       store.New(database),
		vimeo:    vimeo.New(cfg.VimeoAccessToken, cfg.VimeoPrivacy, cfg.VimeoEmbedDomains),
		verifier: verifier,
		log:      log,
	}
	s.routes()
	return s
}

// Handler returns the root handler with CORS and logging applied.
func (s *Server) Handler() http.Handler {
	return httpx.CORS(s.cfg.CORSOrigins)(httpx.Logging(s.log)(s.mux))
}

// handler is an authenticated route: the signed-in user is resolved before it runs.
type handler func(w http.ResponseWriter, r *http.Request, u *model.User) error

// API routes live under /api so the same binary can serve the web app at /.
func (s *Server) routes() {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	a := s.authed
	m.Handle("GET /api/me", a(s.getMe))
	m.Handle("PATCH /api/me", a(s.patchMe))

	m.Handle("GET /api/library", a(s.listLibrary))
	m.Handle("GET /api/library/{creatorId}", a(s.getLibraryTree))

	m.Handle("POST /api/directories", a(s.createDirectory))
	m.Handle("PATCH /api/directories/{id}", a(s.patchDirectory))
	m.Handle("DELETE /api/directories/{id}", a(s.deleteDirectory))

	m.Handle("POST /api/videos/upload-ticket", a(s.uploadTicket))
	m.Handle("POST /api/videos/{id}/complete", a(s.completeVideo))
	m.Handle("GET /api/videos/{id}", a(s.getVideo))
	m.Handle("GET /api/videos/{id}/playback", a(s.playback))
	m.Handle("PATCH /api/videos/{id}", a(s.patchVideo))
	m.Handle("DELETE /api/videos/{id}", a(s.deleteVideo))

	m.Handle("POST /api/share", a(s.createShare))
	m.Handle("GET /api/share", a(s.listShare))
	m.Handle("DELETE /api/share/{id}", a(s.revokeShare))

	m.Handle("GET /api/access", a(s.listAccess))
	m.Handle("DELETE /api/access/{id}", a(s.revokeAccess))

	m.Handle("POST /api/redeem", a(s.redeem))
	m.Handle("GET /api/audit", a(s.listAudit))

	m.Handle("/", spaHandler(web.Dist))
	s.mux = m
}

// authed resolves the caller to a User row, creating it on first sign-in, then runs h.
func (s *Server) authed(h handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.identify(r)
		if err != nil {
			httpx.WriteError(w, s.log, err)
			return
		}
		if err := h(w, r, u); err != nil {
			httpx.WriteError(w, s.log, err)
		}
	})
}

func (s *Server) identify(r *http.Request) (*model.User, error) {
	ctx := r.Context()
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, httpx.Unauthenticated("Missing bearer token")
	}
	id, err := s.verifier.Verify(ctx, strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		s.log.Info("token rejected", "err", err)
		return nil, httpx.Unauthenticated("Invalid or expired token")
	}
	u, err := s.st.GetUserByAuthUID(ctx, id.AuthUID)
	if errors.Is(err, store.ErrNotFound) {
		return s.st.CreateUser(ctx, id.AuthUID, id.Email, id.Name)
	}
	return u, err
}

// ---- shared helpers ----

func requireCreator(u *model.User) error {
	if u.Role != model.RoleCreator {
		return httpx.Forbidden("Creator account required")
	}
	return nil
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func clientIP(r *http.Request) *string {
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.TrimSpace(strings.Split(xff, ",")[0])
	} else if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	return &ip
}

func creatorSummary(u *model.User, rootID, viewerID string) model.CreatorSummary {
	return model.CreatorSummary{ID: u.ID, Name: u.Name, RootDirectoryID: rootID, Own: u.ID == viewerID}
}

// canView reports whether userID may play the video: owner, or an active grant on it or an ancestor.
func (s *Server) canView(ctx context.Context, userID string, v *model.Video) (bool, error) {
	if v.OwnerID == userID {
		return true, nil
	}
	col, err := access.Load(ctx, s.st, v.OwnerID)
	if err != nil || col == nil {
		return false, err
	}
	dirs, vids, err := s.st.ActiveGrantRefs(ctx, userID, v.OwnerID)
	if err != nil {
		return false, err
	}
	return col.VideoVisible(*v, access.NewGrants(dirs, vids)), nil
}

func (s *Server) ownedDirectory(ctx context.Context, u *model.User, id string) (*model.Directory, error) {
	if !isUUID(id) {
		return nil, httpx.NotFound("Directory")
	}
	d, err := s.st.GetDirectory(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && d.OwnerID != u.ID) {
		return nil, httpx.NotFound("Directory")
	}
	return d, err
}

func (s *Server) ownedVideo(ctx context.Context, u *model.User, id string) (*model.Video, error) {
	if !isUUID(id) {
		return nil, httpx.NotFound("Video")
	}
	v, err := s.st.GetVideo(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && v.OwnerID != u.ID) {
		return nil, httpx.NotFound("Video")
	}
	return v, err
}

// vimeoErr maps client errors to HTTP errors.
func vimeoErr(err error) error {
	if errors.Is(err, vimeo.ErrNotConfigured) {
		return httpx.NewError(http.StatusServiceUnavailable, "vimeo_not_configured", err.Error())
	}
	var ae *vimeo.APIError
	if errors.As(err, &ae) {
		return httpx.NewError(http.StatusBadGateway, "vimeo_error", ae.Error())
	}
	return err
}

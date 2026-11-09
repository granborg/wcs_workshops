// Package server wires routes, authentication and handlers.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/granborg/wcs_workshops/apps/api/internal/access"
	"github.com/granborg/wcs_workshops/apps/api/internal/auth"
	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/httpx"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/internal/notify"
	"github.com/granborg/wcs_workshops/apps/api/internal/store"
	"github.com/granborg/wcs_workshops/apps/api/internal/vimeo"
)

// Server holds the dependencies handlers need.
type Server struct {
	cfg      config.Config
	st       *store.Store
	vimeo    *vimeo.Client
	notifier *notify.Notifier
	verifier auth.Verifier
	log      *slog.Logger
	mux      *http.ServeMux
}

// New builds a Server. verifier may be nil when DevAuthBypass is on and no bearer tokens are expected.
func New(cfg config.Config, database *sql.DB, verifier auth.Verifier, log *slog.Logger) *Server {
	st := store.New(database)
	s := &Server{
		cfg:      cfg,
		st:       st,
		vimeo:    vimeo.New(cfg.VimeoAccessToken, cfg.VimeoPrivacy, cfg.VimeoEmbedDomains),
		notifier: notify.New(st, log),
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

func (s *Server) routes() {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	m.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.cfg.UploadDir))))

	a := s.authed
	m.Handle("GET /me", a(s.getMe))
	m.Handle("PATCH /me", a(s.patchMe))
	m.Handle("PUT /me/banner", a(s.putBanner))
	m.Handle("DELETE /me/banner", a(s.deleteBanner))

	m.Handle("GET /library", a(s.listLibrary))
	m.Handle("GET /library/{creatorId}", a(s.getLibraryTree))

	m.Handle("POST /directories", a(s.createDirectory))
	m.Handle("PATCH /directories/{id}", a(s.patchDirectory))
	m.Handle("DELETE /directories/{id}", a(s.deleteDirectory))

	m.Handle("POST /videos/upload-ticket", a(s.uploadTicket))
	m.Handle("POST /videos/{id}/complete", a(s.completeVideo))
	m.Handle("GET /videos/{id}", a(s.getVideo))
	m.Handle("GET /videos/{id}/playback", a(s.playback))
	m.Handle("PATCH /videos/{id}", a(s.patchVideo))
	m.Handle("DELETE /videos/{id}", a(s.deleteVideo))

	m.Handle("POST /share", a(s.createShare))
	m.Handle("GET /share", a(s.listShare))
	m.Handle("DELETE /share/{id}", a(s.revokeShare))

	m.Handle("GET /access", a(s.listAccess))
	m.Handle("DELETE /access/{id}", a(s.revokeAccess))

	m.Handle("POST /redeem", a(s.redeem))
	m.Handle("GET /audit", a(s.listAudit))

	m.Handle("POST /devices", a(s.registerDevice))
	m.Handle("DELETE /devices/{token}", a(s.removeDevice))

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
	var id auth.Identity

	if dev := strings.TrimSpace(r.Header.Get("X-Dev-User")); s.cfg.DevAuthBypass && dev != "" {
		email := strings.ToLower(dev)
		id = auth.Identity{AuthUID: "dev:" + email, Email: &email, Name: strings.SplitN(email, "@", 2)[0]}
	} else {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return nil, httpx.Unauthenticated("Missing bearer token")
		}
		if s.verifier == nil {
			return nil, httpx.Unauthenticated("Token verification is not configured")
		}
		var err error
		id, err = s.verifier.Verify(ctx, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			s.log.Info("token rejected", "err", err)
			return nil, httpx.Unauthenticated("Invalid or expired token")
		}
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

func (s *Server) bannerURL(path *string) *string {
	if path == nil {
		return nil
	}
	if strings.HasPrefix(*path, "http") {
		return path
	}
	full := s.cfg.PublicAPIURL + *path
	return &full
}

func (s *Server) creatorSummary(u *model.User, rootID, viewerID string) model.CreatorSummary {
	return model.CreatorSummary{ID: u.ID, Name: u.Name, BannerURL: s.bannerURL(u.BannerURL), RootDirectoryID: rootID, Own: u.ID == viewerID}
}

// audit records an event. Failures are logged; they never fail the request.
func (s *Server) audit(ctx context.Context, e model.AuditEvent, detail map[string]any) {
	if detail != nil {
		if b, err := json.Marshal(detail); err == nil {
			str := string(b)
			e.Detail = &str
		}
	}
	if err := s.st.InsertAudit(ctx, e); err != nil {
		s.log.Error("audit insert failed", "err", err, "type", e.Type)
	}
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

// background returns a context detached from the request, for work that outlives the response.
func background() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

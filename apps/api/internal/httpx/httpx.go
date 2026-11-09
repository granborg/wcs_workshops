// Package httpx holds small HTTP helpers: typed errors, JSON I/O, CORS and request logging.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Error is an HTTP-level failure with a stable machine-readable code.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func NotFound(what string) *Error         { return NewError(http.StatusNotFound, "not_found", what+" not found") }
func Forbidden(message string) *Error     { return NewError(http.StatusForbidden, "forbidden", message) }
func BadRequest(message string) *Error    { return NewError(http.StatusBadRequest, "bad_request", message) }
func Conflict(message string) *Error      { return NewError(http.StatusConflict, "conflict", message) }
func Unauthenticated(message string) *Error { return NewError(http.StatusUnauthorized, "unauthenticated", message) }

// WriteJSON writes v as JSON with the given status. A nil v writes only the status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if v == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError maps an error to a JSON response. Unknown errors become 500 and are logged.
func WriteError(w http.ResponseWriter, log *slog.Logger, err error) {
	var he *Error
	if errors.As(err, &he) {
		WriteJSON(w, he.Status, map[string]string{"error": he.Code, "message": he.Message})
		return
	}
	log.Error("request failed", "err", err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal", "message": "Internal server error"})
}

// Decode reads a JSON body (up to 1 MB) into dst.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		return BadRequest("Invalid JSON body: " + err.Error())
	}
	return nil
}

// CORS allows the configured browser origins and answers preflight requests.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Dev-User")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logging writes one line per request.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
		})
	}
}

// Package auth verifies Firebase ID tokens.
package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Identity is what a verified token tells us about the caller.
type Identity struct {
	AuthUID string
	Email   *string
	Name    string
}

// Verifier turns a bearer token into an Identity.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

// Firebase verifies Firebase ID tokens. It initialises lazily.
//
// Credentials, in order of precedence:
//   - FIREBASE_CREDENTIALS_JSON: the service account JSON itself (for hosts whose secrets are env vars).
//   - GOOGLE_APPLICATION_CREDENTIALS: path to the service account file.
//   - FIREBASE_AUTH_EMULATOR_HOST: no credentials needed; the Admin SDK talks to the local emulator.
type Firebase struct {
	projectID string
	once      sync.Once
	client    *fbauth.Client
	err       error
}

// NewFirebase creates an uninitialised verifier. projectID may be empty when credentials carry it.
func NewFirebase(projectID string) *Firebase { return &Firebase{projectID: projectID} }

func (f *Firebase) init(ctx context.Context) {
	var conf *firebase.Config
	if f.projectID != "" {
		conf = &firebase.Config{ProjectID: f.projectID}
	}
	var opts []option.ClientOption
	if raw := os.Getenv("FIREBASE_CREDENTIALS_JSON"); raw != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(raw)))
	}
	app, err := firebase.NewApp(ctx, conf, opts...)
	if err != nil {
		f.err = err
		return
	}
	f.client, f.err = app.Auth(ctx)
}

// Verify checks the token and extracts uid, email and name.
func (f *Firebase) Verify(ctx context.Context, idToken string) (Identity, error) {
	f.once.Do(func() { f.init(ctx) })
	if f.err != nil {
		return Identity{}, fmt.Errorf("firebase not configured: %w", f.err)
	}
	tok, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Identity{}, err
	}
	return identityFromClaims(tok.UID, tok.Claims), nil
}

func identityFromClaims(uid string, claims map[string]any) Identity {
	id := Identity{AuthUID: uid, Name: "New user"}
	if e, ok := claims["email"].(string); ok && e != "" {
		id.Email = &e
		id.Name = strings.SplitN(e, "@", 2)[0]
	}
	if n, ok := claims["name"].(string); ok && n != "" {
		id.Name = n
	}
	return id
}

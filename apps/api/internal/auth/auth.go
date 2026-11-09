// Package auth verifies identity tokens. Firebase Authentication is the only production verifier;
// the development bypass lives in the server package.
package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
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

// Firebase verifies Firebase ID tokens. It initialises lazily using GOOGLE_APPLICATION_CREDENTIALS.
type Firebase struct {
	once   sync.Once
	client *fbauth.Client
	err    error
}

// NewFirebase creates an uninitialised verifier. Nothing happens until the first Verify call.
func NewFirebase() *Firebase { return &Firebase{} }

func (f *Firebase) init(ctx context.Context) {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		f.err = err
		return
	}
	f.client, f.err = app.Auth(ctx)
}

// Verify checks the token signature and expiry and extracts uid, email and name.
func (f *Firebase) Verify(ctx context.Context, idToken string) (Identity, error) {
	f.once.Do(func() { f.init(ctx) })
	if f.err != nil {
		return Identity{}, fmt.Errorf("firebase not configured: %w", f.err)
	}
	tok, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Identity{}, err
	}
	id := Identity{AuthUID: tok.UID, Name: "New user"}
	if e, ok := tok.Claims["email"].(string); ok && e != "" {
		id.Email = &e
		id.Name = strings.SplitN(e, "@", 2)[0]
	}
	if n, ok := tok.Claims["name"].(string); ok && n != "" {
		id.Name = n
	}
	return id, nil
}

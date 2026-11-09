package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Emulator tokens are unsigned JWTs. The Admin SDK accepts them when FIREBASE_AUTH_EMULATOR_HOST is set,
// which is how local development signs in without a real Firebase project.
func emulatorToken(t *testing.T, project string, claims map[string]any) string {
	t.Helper()
	now := time.Now().Unix()
	payload := map[string]any{
		"iss":       "https://securetoken.google.com/" + project,
		"aud":       project,
		"iat":       now,
		"exp":       now + 3600,
		"auth_time": now,
	}
	for k, v := range claims {
		payload[k] = v
	}
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]string{"alg": "none", "typ": "JWT"}) + "." + enc(payload) + "."
}

// fakeEmulator answers the accounts:lookup call the SDK makes after parsing an emulator token.
func fakeEmulator(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "accounts:lookup") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"localId":"uid-123","email":"pat@example.com"}]}`))
	}))
	t.Cleanup(ts.Close)
	return strings.TrimPrefix(ts.URL, "http://")
}

func TestVerifyAgainstEmulatorNeedsNoCredentials(t *testing.T) {
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", fakeEmulator(t))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	v := NewFirebase("demo-wcs")
	email := "pat@example.com"
	id, err := v.Verify(context.Background(), emulatorToken(t, "demo-wcs", map[string]any{
		"sub": "uid-123", "user_id": "uid-123", "email": email, "name": "Pat",
	}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id.AuthUID != "uid-123" || id.Email == nil || *id.Email != email || id.Name != "Pat" {
		t.Fatalf("identity = %+v", id)
	}

	if _, err := v.Verify(context.Background(), emulatorToken(t, "other-project", map[string]any{"sub": "x"})); err == nil {
		t.Fatal("token for another project must be rejected")
	}
}

func TestIdentityFromClaimsFallsBackToEmailLocalPart(t *testing.T) {
	id := identityFromClaims("u", map[string]any{"email": "sam@example.com"})
	if id.Name != "sam" {
		t.Fatalf("name = %q", id.Name)
	}
	if identityFromClaims("u", nil).Name != "New user" {
		t.Fatal("expected default name")
	}
}

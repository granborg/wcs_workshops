package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/granborg/wcs_workshops/apps/api/internal/auth"
	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/db"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
	"github.com/granborg/wcs_workshops/apps/api/web"
)

// End-to-end tests of the HTTP API against a temporary SQLite file.
// Tokens are verified by a fake: the bearer token is the user's email address.
// Vimeo is not configured, so upload endpoints are expected to answer 503.

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (auth.Identity, error) {
	if !strings.Contains(token, "@") {
		return auth.Identity{}, errors.New("bad token")
	}
	email := token
	return auth.Identity{AuthUID: "fake:" + email, Email: &email, Name: strings.SplitN(email, "@", 2)[0]}, nil
}

type client struct {
	t    *testing.T
	base string
}

func newClient(t *testing.T) client {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := config.Config{VimeoPrivacy: "unlisted", CORSOrigins: []string{"http://localhost:5173"}}
	srv := New(cfg, database, fakeVerifier{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return client{t: t, base: ts.URL}
}

func (c client) do(user, method, path string, body, out any) (int, string) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api"+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if user != "" {
		req.Header.Set("Authorization", "Bearer "+user)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if out != nil && res.StatusCode < 300 && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return res.StatusCode, string(raw)
}

func (c client) must(user, method, path string, body, out any) {
	c.t.Helper()
	if st, raw := c.do(user, method, path, body, out); st >= 300 {
		c.t.Fatalf("%s %s -> %d %s", method, path, st, raw)
	}
}

func (c client) expect(want int, user, method, path string, body any) {
	c.t.Helper()
	if st, raw := c.do(user, method, path, body, nil); st != want {
		c.t.Fatalf("%s %s -> %d, want %d (%s)", method, path, st, want, raw)
	}
}

type anyMap = map[string]any

const (
	creator = "creator@example.com"
	viewer  = "viewer@example.com"
)

type fixture struct {
	me                    model.Me
	root                  string
	season, week, private model.DirectoryRecord
}

// setup makes creator@ a creator with Season 1/Week 1 and Private folders.
func setup(t *testing.T, c client) fixture {
	t.Helper()
	var f fixture
	c.must(creator, "GET", "/me", nil, &f.me)
	if f.me.Role != "viewer" {
		t.Fatalf("new user role = %q, want viewer", f.me.Role)
	}
	c.must(creator, "PATCH", "/me", anyMap{"role": "creator", "name": "Smoke Creator"}, &f.me)
	if f.me.Role != "creator" || f.me.RootDirectoryID == nil {
		t.Fatalf("after switch: role=%q root=%v", f.me.Role, f.me.RootDirectoryID)
	}
	f.root = *f.me.RootDirectoryID
	c.must(creator, "POST", "/directories", anyMap{"name": "Season 1", "parentId": f.root}, &f.season)
	c.must(creator, "POST", "/directories", anyMap{"name": "Week 1", "parentId": f.season.ID}, &f.week)
	c.must(creator, "POST", "/directories", anyMap{"name": "Private", "parentId": f.root}, &f.private)
	return f
}

func TestDirectoryRules(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)
	c.expect(400, creator, "PATCH", "/directories/"+f.season.ID, anyMap{"parentId": f.week.ID})
	c.expect(400, creator, "DELETE", "/directories/"+f.root+"?mode=delete", nil)
	c.expect(503, creator, "POST", "/videos/upload-ticket", anyMap{"title": "x", "directoryId": f.root, "sizeBytes": 10})
	c.expect(403, viewer, "POST", "/directories", anyMap{"name": "Nope", "parentId": f.root})
	c.expect(400, creator, "POST", "/share", anyMap{"directoryId": f.season.ID, "videoId": f.season.ID})
	c.expect(400, creator, "POST", "/share", anyMap{"directoryId": f.season.ID, "expiresAt": "2000-01-01T00:00:00Z"})
}

func TestShareRedeemAndPrunedLibrary(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.season.ID}, &qr)
	if !strings.HasPrefix(qr.Payload, model.QRScheme) {
		t.Fatalf("payload = %q", qr.Payload)
	}

	c.expect(403, viewer, "GET", "/library/"+f.me.ID, nil)
	c.expect(400, creator, "POST", "/redeem", anyMap{"code": qr.Payload})

	var redeemed model.RedeemResult
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Payload}, &redeemed)
	if !redeemed.Added {
		t.Fatal("first redeem should add")
	}
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Code}, &redeemed)
	if redeemed.Added {
		t.Fatal("second redeem should be idempotent")
	}

	var lib []model.CreatorSummary
	c.must(viewer, "GET", "/library", nil, &lib)
	if len(lib) != 1 || lib[0].ID != f.me.ID || lib[0].Own {
		t.Fatalf("viewer library = %+v", lib)
	}

	var tree model.LibraryTree
	c.must(viewer, "GET", "/library/"+f.me.ID, nil, &tree)
	if tree.CanManage {
		t.Fatal("viewer must not manage")
	}
	byName := map[string]*model.DirectoryNode{}
	for _, n := range tree.Root.Children {
		byName[n.Name] = n
	}
	if byName["Season 1"] == nil || byName["Private"] != nil {
		t.Fatalf("pruned children = %v", keys(byName))
	}
	if len(byName["Season 1"].Children) != 1 || byName["Season 1"].Children[0].Name != "Week 1" {
		t.Fatalf("inherited child missing: %+v", byName["Season 1"].Children)
	}

	var own model.LibraryTree
	c.must(creator, "GET", "/library/"+f.me.ID, nil, &own)
	if !own.CanManage || len(own.Root.Children) != 2 {
		t.Fatalf("owner tree: manage=%v children=%d", own.CanManage, len(own.Root.Children))
	}
}

func TestRevokeAndAudit(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.season.ID}, &qr)
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Code}, nil)

	var grants []model.AccessGrantInfo
	c.must(creator, "GET", "/access?directoryId="+f.season.ID, nil, &grants)
	if len(grants) != 1 || grants[0].User.Name != "viewer" {
		t.Fatalf("grants = %+v", grants)
	}
	c.expect(403, viewer, "DELETE", "/access/"+grants[0].ID, nil)
	c.expect(204, creator, "DELETE", "/access/"+grants[0].ID, nil)
	c.expect(403, viewer, "GET", "/library/"+f.me.ID, nil)

	var revoked model.QrCodeInfo
	c.must(creator, "DELETE", "/share/"+qr.ID, nil, &revoked)
	if revoked.RevokedAt == nil || revoked.Redemptions != 1 {
		t.Fatalf("revoked = %+v", revoked)
	}
	c.expect(404, viewer, "POST", "/redeem", anyMap{"code": qr.Code})

	var audit []model.AuditEventInfo
	c.must(creator, "GET", "/audit", nil, &audit)
	got := map[string]model.AuditEventInfo{}
	for _, e := range audit {
		got[e.Type] = e
	}
	for _, want := range []string{"qr.created", "access.granted", "access.revoked", "qr.revoked"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("audit missing %s; have %v", want, keys(got))
		}
	}
	if g := got["access.granted"]; g.Subject == nil || g.Subject.Name != "viewer" || g.IP == nil || g.QrCodeID == nil {
		t.Fatalf("access.granted lacks subject, ip or code: %+v", g)
	}
	if len(audit) < 2 || audit[0].At < audit[len(audit)-1].At {
		t.Fatalf("audit not newest first: %+v", audit)
	}
	c.expect(403, viewer, "GET", "/audit", nil)

	c.expect(204, creator, "DELETE", "/directories/"+f.season.ID+"?mode=delete", nil)
	var me model.Me
	c.must(creator, "PATCH", "/me", anyMap{"role": "viewer"}, &me)
	if me.Role != "viewer" || me.RootDirectoryID != nil {
		t.Fatalf("back to viewer: %+v", me)
	}
}

func TestUnauthenticated(t *testing.T) {
	c := newClient(t)
	c.expect(401, "", "GET", "/me", nil)
	c.expect(401, "not-a-token", "GET", "/me", nil)
}

func TestCORSPreflight(t *testing.T) {
	c := newClient(t)
	req, _ := http.NewRequest("OPTIONS", c.base+"/api/me", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 204 || res.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("preflight -> %d %q", res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestSPAFallbacks(t *testing.T) {
	c := newClient(t)
	// Unknown API paths are JSON 404s, never the web app.
	res, err := http.Get(c.base + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("/api/nope -> %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	// App routes fall back to index.html when the web app is embedded. Without a build, the root
	// explains what to do instead of serving a blank page.
	res, err = http.Get(c.base + "/library")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	_, builtErr := fs.Stat(web.Dist, "dist/index.html")
	switch {
	case builtErr == nil && (res.StatusCode != 200 || !strings.Contains(string(body), "<div id=\"app\">")):
		t.Fatalf("/library with dist -> %d %q", res.StatusCode, body)
	case builtErr != nil && (res.StatusCode != 404 || !strings.Contains(string(body), "npm run build")):
		t.Fatalf("/library without dist -> %d %q", res.StatusCode, body)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

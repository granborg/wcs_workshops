package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/db"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

// End-to-end test of the access model through the HTTP API, using the dev auth bypass.
// Vimeo is not configured, so upload endpoints are expected to answer 503.

type client struct {
	t    *testing.T
	base string
}

func newClient(t *testing.T) client {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := config.Config{
		DevAuthBypass: true,
		UploadDir:     dir,
		PublicAPIURL:  "http://test",
		VimeoPrivacy:  "unlisted",
		CORSOrigins:   []string{"http://localhost:5173"},
	}
	srv := New(cfg, database, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("X-Dev-User", user)
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

func TestAccessModelEndToEnd(t *testing.T) {
	c := newClient(t)
	creator, viewer := "creator@example.com", "viewer@example.com"

	var health map[string]bool
	c.must(creator, "GET", "/health", nil, &health)
	if !health["ok"] {
		t.Fatal("health not ok")
	}

	var me model.Me
	c.must(creator, "GET", "/me", nil, &me)
	if me.Role != "viewer" {
		t.Fatalf("new user role = %q, want viewer", me.Role)
	}
	c.must(creator, "PATCH", "/me", anyMap{"role": "creator", "name": "Smoke Creator"}, &me)
	if me.Role != "creator" || me.RootDirectoryID == nil {
		t.Fatalf("after switch: role=%q root=%v", me.Role, me.RootDirectoryID)
	}
	rootID := *me.RootDirectoryID

	var season, week, private model.DirectoryRecord
	c.must(creator, "POST", "/directories", anyMap{"name": "Season 1", "parentId": rootID}, &season)
	c.must(creator, "POST", "/directories", anyMap{"name": "Week 1", "parentId": season.ID}, &week)
	c.must(creator, "POST", "/directories", anyMap{"name": "Private", "parentId": rootID}, &private)

	c.expect(400, creator, "PATCH", "/directories/"+season.ID, anyMap{"parentId": week.ID})
	c.expect(400, creator, "DELETE", "/directories/"+rootID+"?mode=delete", nil)
	c.expect(503, creator, "POST", "/videos/upload-ticket", anyMap{"title": "x", "directoryId": rootID, "sizeBytes": 10})
	c.expect(403, viewer, "POST", "/directories", anyMap{"name": "Nope", "parentId": rootID})

	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": season.ID}, &qr)
	if !strings.HasPrefix(qr.Payload, model.QRScheme) {
		t.Fatalf("payload = %q", qr.Payload)
	}
	c.expect(400, creator, "POST", "/share", anyMap{"directoryId": season.ID, "videoId": season.ID})

	c.expect(403, viewer, "GET", "/library/"+me.ID, nil)
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
	if len(lib) != 1 || lib[0].ID != me.ID || lib[0].Own {
		t.Fatalf("viewer library = %+v", lib)
	}

	var tree model.LibraryTree
	c.must(viewer, "GET", "/library/"+me.ID, nil, &tree)
	if tree.CanManage {
		t.Fatal("viewer must not manage")
	}
	names := map[string]*model.DirectoryNode{}
	for _, n := range tree.Root.Children {
		names[n.Name] = n
	}
	if names["Season 1"] == nil || names["Private"] != nil {
		t.Fatalf("pruned children = %v", keys(names))
	}
	if len(names["Season 1"].Children) != 1 || names["Season 1"].Children[0].Name != "Week 1" {
		t.Fatalf("inherited child missing: %+v", names["Season 1"].Children)
	}

	var own model.LibraryTree
	c.must(creator, "GET", "/library/"+me.ID, nil, &own)
	if !own.CanManage || len(own.Root.Children) != 2 {
		t.Fatalf("owner tree: manage=%v children=%d", own.CanManage, len(own.Root.Children))
	}

	var grants []model.AccessGrantInfo
	c.must(creator, "GET", "/access?directoryId="+season.ID, nil, &grants)
	if len(grants) != 1 || grants[0].User.Name != "viewer" {
		t.Fatalf("grants = %+v", grants)
	}
	c.expect(403, viewer, "DELETE", "/access/"+grants[0].ID, nil) // viewers cannot revoke
	c.expect(204, creator, "DELETE", "/access/"+grants[0].ID, nil)
	c.expect(403, viewer, "GET", "/library/"+me.ID, nil)

	var revoked model.QrCodeInfo
	c.must(creator, "DELETE", "/share/"+qr.ID, nil, &revoked)
	if revoked.RevokedAt == nil || revoked.Redemptions != 1 {
		t.Fatalf("revoked = %+v", revoked)
	}
	c.expect(404, viewer, "POST", "/redeem", anyMap{"code": qr.Code})

	c.expect(204, creator, "DELETE", "/directories/"+season.ID+"?mode=delete", nil)
	c.must(creator, "GET", "/library/"+me.ID, nil, &own)
	if len(own.Root.Children) != 1 || own.Root.Children[0].Name != "Private" {
		t.Fatalf("after delete children = %+v", own.Root.Children)
	}

	var audit []model.AuditEventInfo
	c.must(creator, "GET", "/audit", nil, &audit)
	types := map[string]bool{}
	for _, e := range audit {
		types[e.Type] = true
	}
	for _, want := range []string{"qr.created", "access.granted", "access.revoked", "qr.revoked", "directory.deleted"} {
		if !types[want] {
			t.Fatalf("audit missing %s; have %v", want, keys(types))
		}
	}
	c.expect(403, viewer, "GET", "/audit", nil)

	c.must(creator, "PATCH", "/me", anyMap{"role": "viewer"}, &me)
	if me.Role != "viewer" || me.RootDirectoryID != nil {
		t.Fatalf("back to viewer: %+v", me)
	}
}

func TestExpiredCodeIsRefused(t *testing.T) {
	c := newClient(t)
	var me model.Me
	c.must("c@example.com", "PATCH", "/me", anyMap{"role": "creator"}, &me)
	c.expect(400, "c@example.com", "POST", "/share", anyMap{"directoryId": *me.RootDirectoryID, "expiresAt": "2000-01-01T00:00:00Z"})
}

func TestUnauthenticated(t *testing.T) {
	c := newClient(t)
	req, _ := http.NewRequest("GET", c.base+"/me", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("no header -> %d, want 401", res.StatusCode)
	}
}

func TestCORSPreflight(t *testing.T) {
	c := newClient(t)
	req, _ := http.NewRequest("OPTIONS", c.base+"/me", nil)
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

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

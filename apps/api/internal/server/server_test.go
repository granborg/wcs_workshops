package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

type fakeVerifier struct {
	mu      sync.Mutex
	deleted []string // auth uids passed to DeleteUser
}

func (*fakeVerifier) Verify(_ context.Context, token string) (auth.Identity, error) {
	if !strings.Contains(token, "@") {
		return auth.Identity{}, errors.New("bad token")
	}
	email := token
	return auth.Identity{AuthUID: "fake:" + email, Email: &email, Name: strings.SplitN(email, "@", 2)[0]}, nil
}

func (f *fakeVerifier) DeleteUser(_ context.Context, authUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, authUID)
	return nil
}

type client struct {
	t    *testing.T
	base string
	db   *sql.DB // for the few states the HTTP API cannot reach without Vimeo
	auth *fakeVerifier
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
	verifier := &fakeVerifier{}
	srv := New(cfg, database, verifier, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return client{t: t, base: ts.URL, db: database, auth: verifier}
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

// A placeholder is a video with no file. Viewers with access see it; only its creator can upload to it.
func TestPlaceholder(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	c.expect(403, viewer, "POST", "/videos/placeholder", anyMap{"title": "Later", "directoryId": f.root})
	c.expect(400, creator, "POST", "/videos/placeholder", anyMap{"title": " ", "directoryId": f.season.ID})

	var v model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Week 2", "directoryId": f.season.ID}, &v)
	if v.Status != model.StatusPlaceholder || v.OwnerID != f.me.ID {
		t.Fatalf("placeholder = %+v", v)
	}

	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"videoId": v.ID}, &qr)
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Code}, nil)

	var seen model.VideoSummary
	c.must(viewer, "GET", "/videos/"+v.ID, nil, &seen)
	if seen.Status != model.StatusPlaceholder {
		t.Fatalf("viewer sees status %q", seen.Status)
	}
	c.expect(400, viewer, "GET", "/videos/"+v.ID+"/playback", nil)
	c.expect(403, viewer, "POST", "/videos/"+v.ID+"/upload-ticket", anyMap{"sizeBytes": 10})
	// Only the owner and admins can download, and only a video that has a file.
	c.expect(404, viewer, "GET", "/videos/"+v.ID+"/downloads", nil)
	c.expect(400, creator, "GET", "/videos/"+v.ID+"/downloads", nil)

	c.expect(400, creator, "POST", "/videos/"+v.ID+"/upload-ticket", anyMap{"sizeBytes": 0})
	c.expect(503, creator, "POST", "/videos/"+v.ID+"/upload-ticket", anyMap{"sizeBytes": 10})
	c.must(creator, "GET", "/videos/"+v.ID, nil, &seen)
	if seen.Status != model.StatusPlaceholder {
		t.Fatalf("failed ticket changed status to %q", seen.Status)
	}
}

// A video has an optional date. The creator sets it on create and changes or clears it later.
func TestVideoDate(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	c.expect(400, creator, "POST", "/videos/placeholder", anyMap{"title": "Bad", "directoryId": f.root, "videoDate": "2026-02-30"})
	c.expect(400, creator, "POST", "/videos/placeholder", anyMap{"title": "Bad", "directoryId": f.root, "videoDate": ""})

	var v model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Dated", "directoryId": f.root, "videoDate": "2026-03-14"}, &v)
	if v.VideoDate == nil || *v.VideoDate != "2026-03-14" {
		t.Fatalf("videoDate = %v", v.VideoDate)
	}
	var undated model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Undated", "directoryId": f.root}, &undated)
	if undated.VideoDate != nil {
		t.Fatalf("videoDate = %q, want null", *undated.VideoDate)
	}

	c.expect(400, creator, "PATCH", "/videos/"+v.ID, anyMap{"videoDate": "14/03/2026"})
	c.must(creator, "PATCH", "/videos/"+v.ID, anyMap{"videoDate": "2025-12-31"}, &v)
	if v.VideoDate == nil || *v.VideoDate != "2025-12-31" || v.Title != "Dated" {
		t.Fatalf("after patch: %+v", v)
	}
	c.must(creator, "PATCH", "/videos/"+v.ID, anyMap{"videoDate": ""}, &v)
	if v.VideoDate != nil {
		t.Fatalf("after clear: videoDate = %q", *v.VideoDate)
	}
	c.expect(403, viewer, "PATCH", "/videos/"+v.ID, anyMap{"videoDate": "2025-12-31"})
}

// A user can delete their account. A creator must delete their videos first.
func TestDeleteAccount(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	var v model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Keep", "directoryId": f.week.ID}, &v)
	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.season.ID}, &qr)
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Code}, nil)

	c.expect(409, creator, "DELETE", "/me", nil)
	if len(c.auth.deleted) != 0 {
		t.Fatalf("sign-in account deleted while videos remain: %v", c.auth.deleted)
	}
	c.must(creator, "GET", "/library/"+f.me.ID, nil, nil) // nothing was removed

	// A viewer can always delete. Their access grants go with them; the creator's collection stays.
	c.expect(204, viewer, "DELETE", "/me", nil)
	var grants int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM access_grants`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatalf("%d access grants left after the viewer was deleted", grants)
	}

	c.must(creator, "DELETE", "/videos/"+v.ID, nil, nil)
	c.expect(204, creator, "DELETE", "/me", nil)
	if !slices.Equal(c.auth.deleted, []string{"fake:" + viewer, "fake:" + creator}) {
		t.Fatalf("deleted sign-in accounts = %v", c.auth.deleted)
	}
	for _, table := range []string{"users", "directories", "qr_codes"} {
		var n int
		if err := c.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%d rows left in %s", n, table)
		}
	}
}

// An admin of Season 1 manages videos, codes and access there and below, but not elsewhere, and not folders.
func TestCollectionAdmins(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)
	const admin = "admin@example.com"
	var adminMe model.Me
	c.must(admin, "PATCH", "/me", anyMap{"role": "creator"}, &adminMe)

	add := anyMap{"directoryId": f.season.ID, "email": "ADMIN@example.com"}
	c.expect(403, viewer, "POST", "/admins", add)
	c.expect(404, admin, "POST", "/admins", add) // only the owner adds admins
	c.expect(400, creator, "POST", "/admins", anyMap{"directoryId": f.season.ID, "email": viewer})
	c.expect(400, creator, "POST", "/admins", anyMap{"directoryId": f.season.ID, "email": creator})
	var a model.AdminInfo
	c.must(creator, "POST", "/admins", add, &a)
	if a.User.ID != adminMe.ID {
		t.Fatalf("admin = %+v", a)
	}
	var admins []model.AdminInfo
	c.must(creator, "GET", "/admins?directoryId="+f.season.ID, nil, &admins)
	if len(admins) != 1 {
		t.Fatalf("admins = %+v", admins)
	}

	var lib []model.CreatorSummary
	c.must(admin, "GET", "/library", nil, &lib)
	if len(lib) != 2 || lib[1].ID != f.me.ID || !lib[1].Admin {
		t.Fatalf("admin library = %+v", lib)
	}
	var tree model.LibraryTree
	c.must(admin, "GET", "/library/"+f.me.ID, nil, &tree)
	if tree.CanManage || len(tree.AdminDirectoryIDs) != 1 || len(tree.Root.Children) != 1 || tree.Root.Children[0].Name != "Season 1" {
		t.Fatalf("admin tree: manage=%v admin=%v children=%d", tree.CanManage, tree.AdminDirectoryIDs, len(tree.Root.Children))
	}

	// Videos: add under Season 1 (also in Week 1, below it), not in Private.
	var v model.VideoSummary
	c.must(admin, "POST", "/videos/placeholder", anyMap{"title": "Filmed", "directoryId": f.week.ID}, &v)
	if v.OwnerID != f.me.ID {
		t.Fatalf("video owner = %q, want the collection owner", v.OwnerID)
	}
	c.expect(404, admin, "POST", "/videos/placeholder", anyMap{"title": "No", "directoryId": f.private.ID})
	c.must(admin, "GET", "/videos/"+v.ID, nil, &v)
	if !v.CanManage {
		t.Fatal("admin should manage the video")
	}
	c.expect(404, admin, "PATCH", "/videos/"+v.ID, anyMap{"directoryId": f.private.ID})
	c.expect(404, admin, "PATCH", "/videos/"+v.ID, anyMap{"directoryId": adminMe.RootDirectoryID})
	c.must(admin, "PATCH", "/videos/"+v.ID, anyMap{"directoryId": f.season.ID}, nil)

	// Folders stay with the owner.
	c.expect(404, admin, "POST", "/directories", anyMap{"name": "Nope", "parentId": f.season.ID})

	// Codes and access.
	var qr model.QrCodeInfo
	c.must(admin, "POST", "/share", anyMap{"videoId": v.ID}, &qr)
	c.expect(404, admin, "POST", "/share", anyMap{"directoryId": f.private.ID})
	c.must(viewer, "POST", "/redeem", anyMap{"code": qr.Code}, nil)
	var grants []model.AccessGrantInfo
	c.must(admin, "GET", "/access?videoId="+v.ID, nil, &grants)
	if len(grants) != 1 {
		t.Fatalf("grants = %+v", grants)
	}
	c.expect(204, admin, "DELETE", "/access/"+grants[0].ID, nil)
	c.must(admin, "DELETE", "/share/"+qr.ID, nil, nil)
	var codes []model.QrCodeInfo
	c.must(creator, "GET", "/share?videoId="+v.ID, nil, &codes)
	if len(codes) != 1 || codes[0].RevokedAt == nil {
		t.Fatalf("owner should see the code the admin made and revoked: %+v", codes)
	}

	c.expect(204, admin, "DELETE", "/videos/"+v.ID, nil)

	c.expect(404, admin, "DELETE", "/admins/"+a.ID, nil)
	c.expect(204, creator, "DELETE", "/admins/"+a.ID, nil)
	c.expect(404, admin, "POST", "/videos/placeholder", anyMap{"title": "Late", "directoryId": f.week.ID})
	c.expect(403, admin, "GET", "/library/"+f.me.ID, nil)
}

// Any signed-in user can open an upload link. Vimeo is not configured here, so the upload itself answers 503.
func TestUploadLinks(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)
	var v model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Week 3", "directoryId": f.week.ID}, &v)

	c.expect(403, viewer, "POST", "/upload-links", anyMap{"videoId": v.ID})
	var l model.UploadLinkInfo
	c.must(creator, "POST", "/upload-links", anyMap{"videoId": v.ID}, &l)
	if !strings.HasSuffix(l.URL, model.UploadLinkPath+l.Code) {
		t.Fatalf("url = %q", l.URL)
	}

	var target model.UploadLinkTarget
	c.must(viewer, "GET", "/upload-links/code/"+l.Code, nil, &target)
	if !target.Open || target.VideoTitle != "Week 3" || target.CreatorName != "Smoke Creator" {
		t.Fatalf("target = %+v", target)
	}
	c.expect(400, viewer, "POST", "/upload-links/code/"+l.Code+"/ticket", anyMap{"sizeBytes": 0})
	c.expect(503, viewer, "POST", "/upload-links/code/"+l.Code+"/ticket", anyMap{"sizeBytes": 10})
	c.expect(400, viewer, "POST", "/upload-links/code/"+l.Code+"/complete", nil)
	c.expect(404, viewer, "GET", "/upload-links/code/not-a-real-code", nil)

	// Finishing an upload gives the uploader access to watch the video. Without Vimeo the ticket step fails,
	// so record its effect (the link was used by the viewer) in the database directly.
	c.expect(404, viewer, "GET", "/videos/"+v.ID, nil)
	var viewerMe model.Me
	c.must(viewer, "GET", "/me", nil, &viewerMe)
	if _, err := c.db.Exec(`UPDATE upload_links SET used_by = ?, used_at = 1 WHERE id = ?`, viewerMe.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	c.must(viewer, "POST", "/upload-links/code/"+l.Code+"/complete", nil, &target)
	c.must(viewer, "POST", "/upload-links/code/"+l.Code+"/complete", nil, &target) // a retry adds no second grant
	c.must(viewer, "GET", "/videos/"+v.ID, nil, nil)
	var grants []model.AccessGrantInfo
	c.must(creator, "GET", "/access?videoId="+v.ID, nil, &grants)
	if len(grants) != 1 || grants[0].User.ID != viewerMe.ID || grants[0].QrCodeID != nil {
		t.Fatalf("uploader grant = %+v", grants)
	}

	var links []model.UploadLinkInfo
	c.must(creator, "GET", "/upload-links?videoId="+v.ID, nil, &links)
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	c.expect(403, viewer, "DELETE", "/upload-links/"+l.ID, nil)
	c.must(creator, "DELETE", "/upload-links/"+l.ID, nil, &l)
	if l.RevokedAt == nil {
		t.Fatal("link not revoked")
	}
	c.expect(404, viewer, "GET", "/upload-links/code/"+l.Code, nil)

	var audit []model.AuditEventInfo
	c.must(creator, "GET", "/audit", nil, &audit)
	seen := map[string]bool{}
	for _, e := range audit {
		if strings.HasPrefix(e.Type, "upload_link.") {
			seen[e.Type] = true
			if e.TargetName != "Week 3" {
				t.Errorf("%s targetName = %q", e.Type, e.TargetName)
			}
		}
	}
	if !seen["upload_link.created"] || !seen["upload_link.revoked"] {
		t.Fatalf("audit upload link events = %v", seen)
	}
}

func TestShareRedeemAndPrunedLibrary(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)

	var qr model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.season.ID}, &qr)
	if !strings.Contains(qr.Payload, "://") || !strings.HasSuffix(qr.Payload, model.RedeemPath+qr.Code) {
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

	// Codes for the root folder and for a video, to check the names the audit log shows.
	var rootQR, videoQR model.QrCodeInfo
	var v model.VideoSummary
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.root}, &rootQR)
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Week 2", "directoryId": f.week.ID}, &v)
	c.must(creator, "POST", "/share", anyMap{"videoId": v.ID}, &videoQR)

	var audit []model.AuditEventInfo
	c.must(creator, "GET", "/audit", nil, &audit)
	got := map[string]model.AuditEventInfo{}
	for _, e := range audit {
		want := "Season 1"
		switch {
		case e.QrCodeID != nil && *e.QrCodeID == rootQR.ID:
			want = "Smoke Creator" // a root folder shows as the creator's name
		case e.QrCodeID != nil && *e.QrCodeID == videoQR.ID:
			want = "Week 2"
		default:
			got[e.Type] = e
		}
		if e.TargetName != want {
			t.Errorf("%s targetName = %q, want %q", e.Type, e.TargetName, want)
		}
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

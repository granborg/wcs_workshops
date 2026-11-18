package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/model"
)

// Share links and upload links name their folder or video in the page title and the Open Graph tags,
// so a chat app shows the name in its link preview. Other pages and dead links show only the site name.
func TestLinkPreview(t *testing.T) {
	c := newClient(t)
	f := setup(t, c)
	c.must(creator, "PATCH", "/directories/"+f.week.ID, anyMap{"name": `Week <1> & "more"`}, nil)
	var v model.VideoSummary
	c.must(creator, "POST", "/videos/placeholder", anyMap{"title": "Sugar push", "directoryId": f.week.ID}, &v)

	var folderQR, rootQR, videoQR, revokedQR model.QrCodeInfo
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.week.ID}, &folderQR)
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.root}, &rootQR)
	c.must(creator, "POST", "/share", anyMap{"videoId": v.ID}, &videoQR)
	c.must(creator, "POST", "/share", anyMap{"directoryId": f.season.ID}, &revokedQR)
	c.must(creator, "DELETE", "/share/"+revokedQR.ID, nil, nil)
	var up model.UploadLinkInfo
	c.must(creator, "POST", "/upload-links", anyMap{"videoId": v.ID}, &up)

	// A second server on the same database, with a small index.html in place of the built app.
	s := New(config.Config{VimeoPrivacy: "unlisted"}, c.db, c.auth, slog.New(slog.NewTextHandler(io.Discard, nil)))
	dist := fstest.MapFS{"dist/index.html": {Data: []byte("<head>\n    <title>Test Site</title>\n</head>")}}
	ts := httptest.NewServer(spaHandler(dist, s.linkPreview))
	t.Cleanup(ts.Close)

	get := func(path string) string {
		t.Helper()
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}

	cases := []struct {
		path string
		want []string
	}{
		{"/r/" + folderQR.Code, []string{
			"<title>Week &lt;1&gt; &amp; &#34;more&#34; | Test Site</title>",
			`<meta property="og:title" content="Week &lt;1&gt; &amp; &#34;more&#34;" />`,
			`<meta property="og:description" content="Shared by Smoke Creator on Test Site" />`,
		}},
		{"/r/" + rootQR.Code, []string{`<meta property="og:title" content="Smoke Creator&#39;s collection" />`}},
		{"/r/" + videoQR.Code, []string{"<title>Sugar push | Test Site</title>"}},
		{"/up/" + up.Code, []string{"<title>Upload: Sugar push | Test Site</title>"}},
	}
	for _, tc := range cases {
		body := get(tc.path)
		for _, w := range tc.want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %s in\n%s", tc.path, w, body)
			}
		}
	}

	for _, path := range []string{"/r/" + revokedQR.Code, "/r/no-such-code-here", "/library", "/r/" + folderQR.Code + "/x"} {
		if body := get(path); !strings.Contains(body, "<title>Test Site</title>") || strings.Contains(body, "og:") {
			t.Errorf("%s: want only the site name, got\n%s", path, body)
		}
	}
}

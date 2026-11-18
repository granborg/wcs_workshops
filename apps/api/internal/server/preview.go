package server

import (
	"bytes"
	"html"
	"net/http"
	"strings"
	"time"
)

// pageMeta is what a chat app or a browser tab shows for a link: the title, a line of text and a picture.
// The site name from index.html is added to the title and the description when the page is written.
type pageMeta struct {
	Title       string
	Description string
	Image       string
}

// linkPreview names the folder or video of a share link (/r/<code>) or an upload link (/up/<code>).
// It returns nil for other paths, and for a code that is unknown, revoked or expired, so those links
// show only the site name.
func (s *Server) linkPreview(r *http.Request) *pageMeta {
	kind, code, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if !ok || strings.Contains(code, "/") || len(code) < 8 || len(code) > 200 {
		return nil
	}
	switch kind {
	case "r":
		return s.sharePreview(r, code)
	case "up":
		_, v, err := s.openUploadLink(r.Context(), code)
		if err != nil || v.HasFile() {
			return nil
		}
		owner, err := s.st.GetUser(r.Context(), v.OwnerID)
		if err != nil {
			return nil
		}
		return &pageMeta{Title: "Upload: " + v.Title, Description: "Upload the video file for " + owner.Name}
	}
	return nil
}

func (s *Server) sharePreview(r *http.Request, code string) *pageMeta {
	ctx := r.Context()
	qr, err := s.st.GetQrCodeByCode(ctx, code)
	if err != nil || qr.RevokedAt != nil || (qr.ExpiresAt != nil && !qr.ExpiresAt.After(time.Now())) {
		return nil
	}
	creator, err := s.st.GetUser(ctx, qr.CreatorID)
	if err != nil {
		return nil
	}
	m := &pageMeta{Description: "Shared by " + creator.Name}
	switch {
	case qr.VideoID != nil:
		v, err := s.st.GetVideo(ctx, *qr.VideoID)
		if err != nil {
			return nil
		}
		m.Title = v.Title
		if v.ThumbnailURL != nil {
			m.Image = *v.ThumbnailURL
		}
	case qr.DirectoryID != nil:
		d, err := s.st.GetDirectory(ctx, *qr.DirectoryID)
		if err != nil {
			return nil
		}
		m.Title = d.Name
		if d.ParentID == nil {
			// The root folder is always named "Collection", which says nothing on its own.
			m.Title = creator.Name + "'s collection"
		}
	default:
		return nil
	}
	return m
}

// siteTitle finds the <title> element of index.html and the site name in it. ok is false when there is none.
func siteTitle(index []byte) (tag []byte, name string, ok bool) {
	start := bytes.Index(index, []byte("<title>"))
	if start < 0 {
		return nil, "", false
	}
	end := bytes.Index(index[start:], []byte("</title>"))
	if end < 0 {
		return nil, "", false
	}
	tag = index[start : start+end+len("</title>")]
	name = html.UnescapeString(strings.TrimSpace(string(tag[len("<title>") : len(tag)-len("</title>")])))
	return tag, name, name != ""
}

// withMeta puts the page title and Open Graph tags into index.html in place of its <title>.
func withMeta(index []byte, m *pageMeta) []byte {
	tag, site, ok := siteTitle(index)
	if m == nil || m.Title == "" || !ok {
		return index
	}
	e := html.EscapeString
	var b strings.Builder
	b.WriteString("<title>" + e(m.Title) + " | " + e(site) + "</title>\n")
	b.WriteString(`    <meta property="og:site_name" content="` + e(site) + `" />` + "\n")
	b.WriteString(`    <meta property="og:type" content="website" />` + "\n")
	b.WriteString(`    <meta property="og:title" content="` + e(m.Title) + `" />` + "\n")
	if m.Description != "" {
		d := e(m.Description + " on " + site)
		b.WriteString(`    <meta name="description" content="` + d + `" />` + "\n")
		b.WriteString(`    <meta property="og:description" content="` + d + `" />` + "\n")
	}
	if m.Image != "" {
		b.WriteString(`    <meta property="og:image" content="` + e(m.Image) + `" />` + "\n")
		b.WriteString(`    <meta name="twitter:card" content="summary_large_image" />`)
	} else {
		b.WriteString(`    <meta name="twitter:card" content="summary" />`)
	}
	return bytes.Replace(index, tag, []byte(b.String()), 1)
}

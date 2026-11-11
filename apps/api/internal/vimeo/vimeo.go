// Package vimeo is a small client for the parts of the Vimeo API this service uses.
package vimeo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const baseURL = "https://api.vimeo.com"

// ErrNotConfigured is returned when no access token is set.
var ErrNotConfigured = errors.New("VIMEO_ACCESS_TOKEN is not set")

// APIError is a non-2xx response from Vimeo.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vimeo responded %d: %s", e.Status, e.Body)
}

// Client talks to Vimeo with one server-to-server token.
type Client struct {
	token        string
	privacy      string // "unlisted" or "disable"
	embedDomains []string
	http         *http.Client
}

// New creates a Client. privacy must be "unlisted" or "disable".
func New(token, privacy string, embedDomains []string) *Client {
	return &Client{token: token, privacy: privacy, embedDomains: embedDomains, http: &http.Client{Timeout: 30 * time.Second}}
}

// Video is the subset of Vimeo's video representation we read.
type Video struct {
	URI      string  `json:"uri"`
	Name     string  `json:"name"`
	Link     string  `json:"link"`
	Duration float64 `json:"duration"`
	Status   string  `json:"status"`
	Upload   *struct {
		Approach   string `json:"approach"`
		UploadLink string `json:"upload_link"`
		Status     string `json:"status"`
	} `json:"upload"`
	Transcode *struct {
		Status string `json:"status"`
	} `json:"transcode"`
	Width    int `json:"width"`
	Height   int `json:"height"`
	Pictures *struct {
		// DefaultPicture is true while the picture is Vimeo's gray placeholder, not one from the video.
		DefaultPicture bool `json:"default_picture"`
		Sizes          []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Link   string `json:"link"`
		} `json:"sizes"`
	} `json:"pictures"`
}

// Thumbnail returns a picture from the video at least 640 pixels wide (or the largest there is), or "" when
// Vimeo has not made one yet.
func (v *Video) Thumbnail() string {
	if v.Pictures == nil || v.Pictures.DefaultPicture {
		return ""
	}
	best, bestWidth := "", 0
	for _, s := range v.Pictures.Sizes {
		if s.Link == "" {
			continue
		}
		switch {
		case best == "",
			bestWidth < 640 && s.Width > bestWidth,
			s.Width >= 640 && s.Width < bestWidth:
			best, bestWidth = s.Link, s.Width
		}
	}
	return best
}

// Created describes a freshly reserved upload.
type Created struct {
	VimeoID    string
	Hash       *string
	UploadLink string
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c.token == "" {
		return ErrNotConfigured
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.vimeo.*+json;version=3.4")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg := string(data)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &APIError{Status: res.StatusCode, Body: msg}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) privacyBody() map[string]any {
	if c.privacy == "disable" {
		return map[string]any{
			"privacy":       map[string]any{"view": "disable", "embed": "whitelist", "download": false, "add": false, "comments": "nobody"},
			"embed_domains": c.embedDomains,
		}
	}
	return map[string]any{
		"privacy": map[string]any{"view": "unlisted", "embed": "public", "download": false, "add": false, "comments": "nobody"},
	}
}

// CreateTusUpload reserves a video and returns the one-time tus upload link for the device.
func (c *Client) CreateTusUpload(ctx context.Context, name string, sizeBytes int64) (*Created, error) {
	body := c.privacyBody()
	body["name"] = name
	body["upload"] = map[string]any{"approach": "tus", "size": fmt.Sprint(sizeBytes)}
	var v Video
	if err := c.do(ctx, http.MethodPost, "/me/videos", body, &v); err != nil {
		return nil, err
	}
	if v.Upload == nil || v.Upload.UploadLink == "" {
		return nil, &APIError{Status: 502, Body: "Vimeo did not return an upload link"}
	}
	id := IDFromURI(v.URI)
	// Also add each domain with its own request, so the allow list does not depend on Vimeo reading
	// embed_domains from the create request. With an empty allow list the video plays nowhere.
	if err := c.allowDomains(ctx, id); err != nil {
		return nil, err
	}
	return &Created{VimeoID: id, Hash: HashFromLink(v.Link), UploadLink: v.Upload.UploadLink}, nil
}

// Privacy is what Vimeo reports for one video.
type Privacy struct {
	View    string
	Embed   string
	Domains []string
}

// Account returns the plan of the account that owns the token, for example "free", "basic" or "plus".
func (c *Client) Account(ctx context.Context) (name, account string, err error) {
	var me struct {
		Name    string `json:"name"`
		Account string `json:"account"`
	}
	if err := c.do(ctx, http.MethodGet, "/me?fields=name,account", nil, &me); err != nil {
		return "", "", err
	}
	return me.Name, me.Account, nil
}

// GetPrivacy reads the view and embed settings of a video, and its embed allow list.
func (c *Client) GetPrivacy(ctx context.Context, vimeoID string) (*Privacy, error) {
	var v struct {
		Privacy struct {
			View  string `json:"view"`
			Embed string `json:"embed"`
		} `json:"privacy"`
	}
	path := "/videos/" + url.PathEscape(vimeoID)
	if err := c.do(ctx, http.MethodGet, path+"?fields=privacy.view,privacy.embed", nil, &v); err != nil {
		return nil, err
	}
	p := &Privacy{View: v.Privacy.View, Embed: v.Privacy.Embed, Domains: []string{}}
	if p.Embed != "whitelist" {
		return p, nil
	}
	var list struct {
		Data []struct {
			Domain string `json:"domain"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path+"/privacy/domains?per_page=100", nil, &list); err != nil {
		return nil, err
	}
	for _, d := range list.Data {
		p.Domains = append(p.Domains, d.Domain)
	}
	return p, nil
}

// ApplyPrivacy sets the configured privacy on an existing video, the same as a new upload gets.
func (c *Client) ApplyPrivacy(ctx context.Context, vimeoID string) error {
	if err := c.do(ctx, http.MethodPatch, "/videos/"+url.PathEscape(vimeoID), c.privacyBody(), nil); err != nil {
		return err
	}
	return c.allowDomains(ctx, vimeoID)
}

// Wants reports whether p matches the configured privacy. Extra allowed domains are not a mismatch.
func (c *Client) Wants(p *Privacy) bool {
	want := c.privacyBody()["privacy"].(map[string]any)
	if p.View != want["view"] || p.Embed != want["embed"] {
		return false
	}
	if c.privacy != "disable" {
		return true
	}
	for _, d := range c.embedDomains {
		if !slices.Contains(p.Domains, d) {
			return false
		}
	}
	return true
}

// allowDomains adds each configured embed domain to the video's allow list. Only "disable" mode uses the list.
func (c *Client) allowDomains(ctx context.Context, vimeoID string) error {
	if c.privacy != "disable" {
		return nil
	}
	for _, d := range c.embedDomains {
		path := "/videos/" + url.PathEscape(vimeoID) + "/privacy/domains/" + url.PathEscape(d)
		if err := c.do(ctx, http.MethodPut, path, nil, nil); err != nil {
			return fmt.Errorf("allow embed domain %s: %w", d, err)
		}
	}
	return nil
}

// GetVideo fetches status fields for one video.
func (c *Client) GetVideo(ctx context.Context, vimeoID string) (*Video, error) {
	var v Video
	err := c.do(ctx, http.MethodGet,
		"/videos/"+url.PathEscape(vimeoID)+"?fields=uri,name,link,duration,status,upload.status,transcode.status,width,height,"+
			"pictures.default_picture,pictures.sizes", nil, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Rename changes the title on Vimeo.
func (c *Client) Rename(ctx context.Context, vimeoID, name string) error {
	return c.do(ctx, http.MethodPatch, "/videos/"+url.PathEscape(vimeoID), map[string]any{"name": name}, nil)
}

// Delete removes the video from Vimeo.
func (c *Client) Delete(ctx context.Context, vimeoID string) error {
	return c.do(ctx, http.MethodDelete, "/videos/"+url.PathEscape(vimeoID), nil, nil)
}

// IDFromURI turns "/videos/123456" into "123456".
func IDFromURI(uri string) string {
	parts := strings.Split(uri, "/")
	return parts[len(parts)-1]
}

// HashFromLink turns "https://vimeo.com/123456/abcdef1234" into "abcdef1234", or nil when there is no hash.
func HashFromLink(link string) *string {
	u, err := url.Parse(link)
	if err != nil {
		return nil
	}
	parts := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	if len(parts) < 2 {
		return nil
	}
	h := parts[1]
	return &h
}

// EmbedURL builds the player URL, including the unlisted hash when present.
func EmbedURL(vimeoID string, hash *string) string {
	q := url.Values{}
	if hash != nil {
		q.Set("h", *hash)
	}
	q.Set("dnt", "1")
	return "https://player.vimeo.com/video/" + url.PathEscape(vimeoID) + "?" + q.Encode()
}

// Download is one size of a video file. Vimeo gives these signed, time-limited links to the video's owner.
type Download struct {
	Quality    string `json:"quality"`   // for example "source", "hd" or "sd"
	Rendition  string `json:"rendition"` // for example "source", "1080p" or "720p"
	PublicName string `json:"public_name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Size       int64  `json:"size"`
	Link       string `json:"link"`
	Expires    string `json:"expires"`
}

// Downloads lists the sizes of a video that the account can download.
func (c *Client) Downloads(ctx context.Context, vimeoID string) ([]Download, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/videos/"+url.PathEscape(vimeoID)+"?fields=download", nil, &raw); err != nil {
		return nil, err
	}
	// Vimeo sends [] instead of {} when none of the requested fields has a value: there is no download.
	body := bytes.TrimSpace(raw)
	if len(body) == 0 || string(body) == "[]" {
		return []Download{}, nil
	}
	var v struct {
		Download []Download `json:"download"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("unexpected download response %q: %w", msg, err)
	}
	return v.Download, nil
}

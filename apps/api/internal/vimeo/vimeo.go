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
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Status   string  `json:"status"`
	Pictures *struct {
		// Type is "default" while Vimeo still serves its generic placeholder picture.
		Type  string `json:"type"`
		Sizes []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Link   string `json:"link"`
		} `json:"sizes"`
	} `json:"pictures"`
	Upload *struct {
		Approach   string `json:"approach"`
		UploadLink string `json:"upload_link"`
		Status     string `json:"status"`
	} `json:"upload"`
	Transcode *struct {
		Status string `json:"status"`
	} `json:"transcode"`
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
	return &Created{VimeoID: IDFromURI(v.URI), Hash: HashFromLink(v.Link), UploadLink: v.Upload.UploadLink}, nil
}

// GetVideo fetches status fields for one video.
func (c *Client) GetVideo(ctx context.Context, vimeoID string) (*Video, error) {
	var v Video
	err := c.do(ctx, http.MethodGet,
		"/videos/"+url.PathEscape(vimeoID)+
			"?fields=uri,name,link,duration,width,height,status,upload.status,transcode.status,pictures.type,pictures.sizes",
		nil, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Thumbnail picks a list-sized picture link: the smallest one at least 400px wide, else the largest.
// Nil while Vimeo still serves its placeholder picture or has no picture yet.
func (v *Video) Thumbnail() *string {
	if v.Pictures == nil || v.Pictures.Type == "default" || len(v.Pictures.Sizes) == 0 {
		return nil
	}
	best := v.Pictures.Sizes[0]
	for _, s := range v.Pictures.Sizes[1:] {
		if s.Width >= 400 && (best.Width < 400 || s.Width < best.Width) || best.Width < 400 && s.Width > best.Width {
			best = s
		}
	}
	if best.Link == "" {
		return nil
	}
	return &best.Link
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

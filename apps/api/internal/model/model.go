// Package model holds database row types and the JSON shapes shared with the app
// (mirrored in apps/app/src/types.ts).
package model

import "time"

const (
	RoleViewer  = "viewer"
	RoleCreator = "creator"
)

const (
	// StatusPlaceholder is a video with no file yet. The creator uploads the file later.
	StatusPlaceholder = "placeholder"
	StatusUploading   = "uploading"
	StatusProcessing  = "processing"
	StatusReady       = "ready"
	StatusFailed      = "failed"
)

// QRScheme is the custom-scheme payload that early QR codes encoded. Only the installed Android app opens it,
// so new codes encode an https link (see QrCode.ToInfo). Redeem still accepts it for codes already printed.
const QRScheme = "wcsw://redeem/"

// RedeemPath is the web route a QR link points to: <public URL>/r/<code>.
const RedeemPath = "/r/"

// UploadLinkPath is the web route an upload link points to: <public URL>/up/<code>.
const UploadLinkPath = "/up/"

// ISO formats a time like JavaScript's Date#toISOString.
func ISO(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z07:00") }

// ISOPtr is ISO for optional times.
func ISOPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := ISO(*t)
	return &s
}

// ---- rows ----

type User struct {
	ID        string
	AuthUID   string
	Email     *string
	Name      string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Directory struct {
	ID        string
	OwnerID   string
	ParentID  *string
	Name      string
	CreatedAt time.Time
}

type Video struct {
	ID          string
	OwnerID     string
	DirectoryID string
	Title       string
	VimeoID     *string
	VimeoHash   *string
	Status      string
	DurationSec *int
	// Width, Height and ThumbnailURL come from Vimeo once the video is processed. Nil until then.
	Width        *int
	Height       *int
	ThumbnailURL *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type QrCode struct {
	ID          string
	Code        string
	CreatorID   string
	DirectoryID *string
	VideoID     *string
	CreatedIP   *string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	Redemptions int
}

// CollectionAdmin lets another creator manage a folder (and everything under it) of the owner's collection.
type CollectionAdmin struct {
	ID          string
	OwnerID     string
	DirectoryID string
	UserID      string
	CreatedAt   time.Time
}

// UploadLink lets any signed-in user upload the file of one video that has no file yet.
type UploadLink struct {
	ID         string
	Code       string
	OwnerID    string
	VideoID    string
	CreatedIP  *string
	UsedBy     *string
	UsedByName *string
	UsedIP     *string
	UsedAt     *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type AccessGrant struct {
	ID          string
	UserID      string
	CreatorID   string
	DirectoryID *string
	VideoID     *string
	QrCodeID    *string
	IP          *string
	CreatedAt   time.Time
	RevokedAt   *time.Time
}

// ---- JSON shapes ----

type Me struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Email           *string `json:"email"`
	Role            string  `json:"role"`
	RootDirectoryID *string `json:"rootDirectoryId"`
}

type CreatorSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	RootDirectoryID string `json:"rootDirectoryId"`
	Own             bool   `json:"own"`
	// Admin is true when the signed-in user is an admin of at least one folder of this collection.
	Admin bool `json:"admin"`
}

type VideoSummary struct {
	ID          string `json:"id"`
	OwnerID     string `json:"ownerId"`
	Title       string `json:"title"`
	DirectoryID string `json:"directoryId"`
	Status      string `json:"status"`
	DurationSec *int   `json:"durationSec"`
	// Frame size in pixels and a thumbnail link, null until Vimeo has processed the video.
	Width        *int    `json:"width"`
	Height       *int    `json:"height"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	CreatedAt    string  `json:"createdAt"`
	// CanManage is set only by GET /videos/{id}: the signed-in user is the owner or an admin of the video.
	CanManage bool `json:"canManage"`
}

type DirectoryNode struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	ParentID *string          `json:"parentId"`
	Children []*DirectoryNode `json:"children"`
	Videos   []VideoSummary   `json:"videos"`
}

type LibraryTree struct {
	Creator CreatorSummary `json:"creator"`
	// CanManage is true for the owner, who can also manage folders and admins.
	CanManage bool `json:"canManage"`
	// AdminDirectoryIDs are the folders where the signed-in user is an admin. The user manages the videos,
	// share codes and access of these folders and of every folder under them.
	AdminDirectoryIDs []string       `json:"adminDirectoryIds"`
	Root              *DirectoryNode `json:"root"`
}

type DirectoryRecord struct {
	ID        string  `json:"id"`
	OwnerID   string  `json:"ownerId"`
	ParentID  *string `json:"parentId"`
	Name      string  `json:"name"`
	CreatedAt string  `json:"createdAt"`
}

type QrCodeInfo struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Payload     string  `json:"payload"`
	DirectoryID *string `json:"directoryId"`
	VideoID     *string `json:"videoId"`
	ExpiresAt   *string `json:"expiresAt"`
	RevokedAt   *string `json:"revokedAt"`
	CreatedAt   string  `json:"createdAt"`
	Redemptions int     `json:"redemptions"`
}

type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AccessGrantInfo struct {
	ID          string  `json:"id"`
	User        UserRef `json:"user"`
	DirectoryID *string `json:"directoryId"`
	VideoID     *string `json:"videoId"`
	QrCodeID    *string `json:"qrCodeId"`
	CreatedAt   string  `json:"createdAt"`
}

type AuditEventInfo struct {
	ID          string   `json:"id"`
	At          string   `json:"at"`
	Type        string   `json:"type"`
	Subject     *UserRef `json:"subject"`
	IP          *string  `json:"ip"`
	DirectoryID *string  `json:"directoryId"`
	VideoID     *string  `json:"videoId"`
	// TargetName is the folder name or video title, as the app shows it. A root folder shows as the creator's name.
	TargetName string  `json:"targetName"`
	QrCodeID   *string `json:"qrCodeId"`
}

type AdminInfo struct {
	ID          string  `json:"id"`
	User        UserRef `json:"user"`
	Email       *string `json:"email"`
	DirectoryID string  `json:"directoryId"`
	CreatedAt   string  `json:"createdAt"`
}

// UploadLinkInfo is an upload link as its owner or an admin sees it.
type UploadLinkInfo struct {
	ID        string   `json:"id"`
	Code      string   `json:"code"`
	URL       string   `json:"url"`
	VideoID   string   `json:"videoId"`
	UsedBy    *UserRef `json:"usedBy"`
	UsedAt    *string  `json:"usedAt"`
	RevokedAt *string  `json:"revokedAt"`
	CreatedAt string   `json:"createdAt"`
}

// UploadLinkTarget is what the person who opens an upload link sees.
type UploadLinkTarget struct {
	VideoID     string `json:"videoId"`
	VideoTitle  string `json:"videoTitle"`
	CreatorName string `json:"creatorName"`
	// Open is false when the video already has a file: the link no longer accepts uploads.
	Open bool `json:"open"`
}

type UploadTicket struct {
	VideoID    string `json:"videoId"`
	UploadLink string `json:"uploadLink"`
}

type PlaybackInfo struct {
	VideoID  string  `json:"videoId"`
	VimeoID  string  `json:"vimeoId"`
	Hash     *string `json:"hash"`
	EmbedURL string  `json:"embedUrl"`
}

type RedeemResult struct {
	Creator     CreatorSummary `json:"creator"`
	DirectoryID *string        `json:"directoryId"`
	VideoID     *string        `json:"videoId"`
	Added       bool           `json:"added"`
}

// ToSummary converts a video row to its JSON shape.
func (v Video) ToSummary() VideoSummary {
	return VideoSummary{
		ID:          v.ID,
		OwnerID:     v.OwnerID,
		Title:       v.Title,
		DirectoryID: v.DirectoryID,
		Status:      v.Status,
		DurationSec:  v.DurationSec,
		Width:        v.Width,
		Height:       v.Height,
		ThumbnailURL: v.ThumbnailURL,
		CreatedAt:    ISO(v.CreatedAt),
	}
}

// ToRecord converts a directory row to its JSON shape.
func (d Directory) ToRecord() DirectoryRecord {
	return DirectoryRecord{ID: d.ID, OwnerID: d.OwnerID, ParentID: d.ParentID, Name: d.Name, CreatedAt: ISO(d.CreatedAt)}
}

// ToInfo converts a QR code row to its JSON shape. baseURL is the public origin of the web app, for
// example https://wcs-workshops.fly.dev, so any phone camera can open the payload.
func (q QrCode) ToInfo(baseURL string) QrCodeInfo {
	return QrCodeInfo{
		ID:          q.ID,
		Code:        q.Code,
		Payload:     baseURL + RedeemPath + q.Code,
		DirectoryID: q.DirectoryID,
		VideoID:     q.VideoID,
		ExpiresAt:   ISOPtr(q.ExpiresAt),
		RevokedAt:   ISOPtr(q.RevokedAt),
		CreatedAt:   ISO(q.CreatedAt),
		Redemptions: q.Redemptions,
	}
}

// ToInfo converts an upload link row to its JSON shape. baseURL is the public origin of the web app.
func (l UploadLink) ToInfo(baseURL string) UploadLinkInfo {
	info := UploadLinkInfo{
		ID:        l.ID,
		Code:      l.Code,
		URL:       baseURL + UploadLinkPath + l.Code,
		VideoID:   l.VideoID,
		UsedAt:    ISOPtr(l.UsedAt),
		RevokedAt: ISOPtr(l.RevokedAt),
		CreatedAt: ISO(l.CreatedAt),
	}
	if l.UsedBy != nil {
		name := "Deleted user"
		if l.UsedByName != nil {
			name = *l.UsedByName
		}
		info.UsedBy = &UserRef{ID: *l.UsedBy, Name: name}
	}
	return info
}

// HasFile reports whether the video has a file on Vimeo, or one that is on its way.
func (v Video) HasFile() bool {
	return v.Status == StatusProcessing || v.Status == StatusReady
}

package registry

import "time"

// PackageInfo is a summary returned in search/list results.
type PackageInfo struct {
	FullName    string `json:"full_name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Repository  string `json:"repository,omitempty"`
}

// PackageDetail is the full package metadata.
type PackageDetail struct {
	PackageInfo
	License    string            `json:"license,omitempty"`
	Keywords   []string          `json:"keywords,omitempty"`
	Platforms  []string          `json:"platforms,omitempty"`
	Homepage   string            `json:"homepage,omitempty"`
	Author     string            `json:"author,omitempty"`
	Visibility string            `json:"visibility,omitempty"`
	Versions   []VersionSummary  `json:"versions,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// VersionSummary is a brief version listing.
type VersionSummary struct {
	Version   string    `json:"version"`
	Yanked    bool      `json:"yanked"`
	CreatedAt time.Time `json:"created_at"`
}

// VersionDetail is the full version metadata including manifest.
type VersionDetail struct {
	Version     string `json:"version"`
	Manifest    string `json:"manifest"` // JSON-encoded ctx.yaml
	Readme      string `json:"readme,omitempty"`
	SHA256      string `json:"sha256"`
	Yanked      bool   `json:"yanked"`
	PublishedBy string `json:"published_by"`
	CreatedAt   string `json:"created_at"`
}

// SearchResult wraps search response.
type SearchResult struct {
	Packages []PackageInfo `json:"packages"`
	Total    int           `json:"total"`
}

// ResolveRequest is sent to POST /v1/resolve.
type ResolveRequest struct {
	Packages map[string]string `json:"packages"` // "@scope/name": "^1.0"
}

// ResolveResponse contains resolved versions.
type ResolveResponse struct {
	Resolved map[string]ResolvedPackage `json:"resolved"`
}

// ResolvedPackage is one entry in the resolve response.
type ResolvedPackage struct {
	Version     string `json:"version"`
	Manifest    string `json:"manifest"`
	DownloadURL string `json:"download_url"`
	SHA256      string `json:"sha256"`
}

// PublishResponse is returned after a successful publish.
type PublishResponse struct {
	FullName   string            `json:"full_name"`
	Version    string            `json:"version"`
	URL        string            `json:"url"`
	Visibility string            `json:"visibility,omitempty"`
	TrustTier  string            `json:"trust_tier,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// ErrorResponse is the standard API error format.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// UserInfo is the current user.
type UserInfo struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// OrgInfo is a summary of an organization.
type OrgInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// OrgDetail is the full org metadata.
type OrgDetail struct {
	OrgInfo
	Members  int `json:"members"`
	Packages int `json:"packages"`
}

// DistTag is a named pointer to a version.
type DistTag struct {
	Tag     string `json:"tag"`
	Version string `json:"version"`
}

// SyncProfile represents the cross-device sync state.
type SyncProfile struct {
	Version    int                `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Device     string            `json:"device"`
	Packages   []SyncPackageEntry `json:"packages"`
}

// SyncPackageEntry is one installed package in the sync profile.
type SyncPackageEntry struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Source     string   `json:"source"`
	SourceURL  string   `json:"source_url,omitempty"`
	Constraint string   `json:"constraint,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Agents     []string `json:"agents"`
	Syncable   bool     `json:"syncable"`
}

// SyncProfileResponse is returned by GET /v1/me/sync-profile.
type SyncProfileResponse struct {
	Profile SyncProfile         `json:"profile"`
	Meta    SyncProfileMeta     `json:"meta"`
}

// SyncProfileMeta contains sync timing metadata.
type SyncProfileMeta struct {
	PackageCount    int    `json:"package_count"`
	SyncableCount   int    `json:"syncable_count"`
	UnsyncableCount int    `json:"unsyncable_count"`
	LastPushAt      string `json:"last_push_at,omitempty"`
	LastPullAt      string `json:"last_pull_at,omitempty"`
	LastPushDevice  string `json:"last_push_device,omitempty"`
	LastPullDevice  string `json:"last_pull_device,omitempty"`
}

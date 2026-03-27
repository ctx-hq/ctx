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
	License   string            `json:"license,omitempty"`
	Keywords  []string          `json:"keywords,omitempty"`
	Platforms []string          `json:"platforms,omitempty"`
	Homepage  string            `json:"homepage,omitempty"`
	Author    string            `json:"author,omitempty"`
	Versions  []VersionSummary  `json:"versions,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
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
	FullName string `json:"full_name"`
	Version  string `json:"version"`
	URL      string `json:"url"`
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

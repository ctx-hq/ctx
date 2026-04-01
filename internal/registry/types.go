package registry

import (
	"fmt"
	"strings"
	"time"
)

// FlexTime handles both RFC3339 ("2006-01-02T15:04:05Z") and SQLite
// datetime ("2006-01-02 15:04:05") formats returned by the API.
type FlexTime struct {
	time.Time
}

func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		ft.Time = t
		return nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		ft.Time = t.UTC()
		return nil
	}
	return fmt.Errorf("cannot parse time %q", s)
}

func (ft FlexTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + ft.Time.Format(time.RFC3339) + `"`), nil
}

// FlexBool handles both JSON booleans (true/false) and SQLite integers (0/1).
type FlexBool bool

func (fb *FlexBool) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch s {
	case "true", "1":
		*fb = true
	case "false", "0", "null", "":
		*fb = false
	default:
		return fmt.Errorf("cannot parse bool %q", s)
	}
	return nil
}

func (fb FlexBool) MarshalJSON() ([]byte, error) {
	if fb {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

// PackageInfo is a summary returned in search/list results.
type PackageInfo struct {
	FullName    string `json:"full_name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Repository  string `json:"repository,omitempty"`
	OwnerSlug   string `json:"owner_slug,omitempty"`
}

// PackageDetail is the full package metadata.
// OwnerInfo represents the package owner (user, org, or system).
type OwnerInfo struct {
	Slug      string `json:"slug"`
	Kind      string `json:"kind"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type PackageDetail struct {
	PackageInfo
	License    string           `json:"license,omitempty"`
	Keywords   []string         `json:"keywords,omitempty"`
	Platforms  []string         `json:"platforms,omitempty"`
	Homepage   string           `json:"homepage,omitempty"`
	Author     string           `json:"author,omitempty"`
	Visibility string           `json:"visibility,omitempty"`
	Owner      *OwnerInfo       `json:"owner,omitempty"`
	Versions   []VersionSummary `json:"versions,omitempty"`
	CreatedAt  FlexTime         `json:"created_at"`
	UpdatedAt  FlexTime         `json:"updated_at"`
}

// VersionSummary is a brief version listing.
type VersionSummary struct {
	Version   string   `json:"version"`
	Yanked    FlexBool `json:"yanked"`
	CreatedAt FlexTime `json:"created_at"`
}

// VersionDetail is the full version metadata including manifest.
type VersionDetail struct {
	Version     string   `json:"version"`
	Manifest    string   `json:"manifest"` // JSON-encoded ctx.yaml
	Readme      string   `json:"readme,omitempty"`
	SHA256      string   `json:"sha256"`
	Yanked      FlexBool `json:"yanked"`
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

// OrgInvitation represents an org membership invitation.
type OrgInvitation struct {
	ID             string `json:"id"`
	OrgName        string `json:"org_name"`
	OrgDisplayName string `json:"org_display_name,omitempty"`
	Inviter        string `json:"inviter"`
	Invitee        string `json:"invitee"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	ExpiresAt      string `json:"expires_at"`
	CreatedAt      string `json:"created_at"`
	ResolvedAt     string `json:"resolved_at,omitempty"`
}

// PackageAccessEntry represents a user granted access to a restricted package.
type PackageAccessEntry struct {
	Username  string `json:"username"`
	GrantedBy string `json:"granted_by"`
	CreatedAt string `json:"created_at"`
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

// TransferRequest represents a package transfer request.
type TransferRequest struct {
	ID        string `json:"id"`
	Package   string `json:"package"`
	From      string `json:"from"`
	To        string `json:"to"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// Notification represents a user notification.
type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

// RenameResult represents the result of a rename operation.
type RenameResult struct {
	OldName         string `json:"old_name,omitempty"`
	NewName         string `json:"new_name,omitempty"`
	OldUsername     string `json:"old_username,omitempty"`
	NewUsername     string `json:"new_username,omitempty"`
	PackagesUpdated int    `json:"packages_updated,omitempty"`
}

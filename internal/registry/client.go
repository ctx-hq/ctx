package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
)

// Client communicates with the getctx.org API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New creates a new registry client.
// Transport is cloned from http.DefaultTransport to preserve proxy, HTTP/2, and
// connection pooling defaults; only dial/TLS/response-header timeouts are overridden.
// No global http.Client.Timeout is set so that large downloads and uploads can
// stream without being hard-cut — callers control deadlines via context.
func New(baseURL, token string) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = 30 * time.Second

	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: &http.Client{Transport: tr},
	}
}

// Search searches packages.
func (c *Client) Search(ctx context.Context, query string, pkgType string, platform string, limit int) (*SearchResult, error) {
	return c.SearchWithOffset(ctx, query, pkgType, platform, limit, 0)
}

// SearchWithOffset searches the registry with pagination support.
func (c *Client) SearchWithOffset(ctx context.Context, query string, pkgType string, platform string, limit int, offset int) (*SearchResult, error) {
	params := url.Values{"q": {query}}
	if pkgType != "" {
		params.Set("type", pkgType)
	}
	if platform != "" {
		params.Set("platform", platform)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}

	var result SearchResult
	if err := c.get(ctx, "/v1/search?"+params.Encode(), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPackage fetches package detail.
func (c *Client) GetPackage(ctx context.Context, fullName string) (*PackageDetail, error) {
	var result PackageDetail
	if err := c.get(ctx, "/v1/packages/"+url.PathEscape(fullName), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetVersion fetches a specific version.
func (c *Client) GetVersion(ctx context.Context, fullName, version string) (*VersionDetail, error) {
	var result VersionDetail
	path := fmt.Sprintf("/v1/packages/%s/versions/%s", url.PathEscape(fullName), url.PathEscape(version))
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Resolve resolves version constraints.
func (c *Client) Resolve(ctx context.Context, req *ResolveRequest) (*ResolveResponse, error) {
	var result ResolveResponse
	if err := c.post(ctx, "/v1/resolve?include_artifacts=true", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Publish publishes a package (manifest + optional archive).
func (c *Client) Publish(ctx context.Context, manifestData []byte, archive io.Reader, readmeData []byte) (*PublishResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("manifest", "ctx.yaml")
	if err != nil {
		return nil, fmt.Errorf("create manifest part: %w", err)
	}
	if _, err := part.Write(manifestData); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	if archive != nil {
		archivePart, err := writer.CreateFormFile("archive", "package.tar.gz")
		if err != nil {
			return nil, fmt.Errorf("create archive part: %w", err)
		}
		if _, err := io.Copy(archivePart, archive); err != nil {
			return nil, fmt.Errorf("write archive: %w", err)
		}
	}

	// Include README if provided
	if len(readmeData) > 0 {
		readmePart, err := writer.CreateFormFile("readme", "README.md")
		if err != nil {
			return nil, fmt.Errorf("create readme part: %w", err)
		}
		if _, err := readmePart.Write(readmeData); err != nil {
			return nil, fmt.Errorf("write readme: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/packages", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", config.UserAgent())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("publish request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var result PublishResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode publish response: %w", err)
	}
	return &result, nil
}

// Yank yanks a version.
func (c *Client) Yank(ctx context.Context, fullName, version string) error {
	path := fmt.Sprintf("/v1/packages/%s/versions/%s/yank", url.PathEscape(fullName), url.PathEscape(version))
	return c.post(ctx, path, nil, nil)
}

// DeletePackage permanently deletes a package and all its versions.
func (c *Client) DeletePackage(ctx context.Context, fullName string) error {
	path := fmt.Sprintf("/v1/packages/%s", url.PathEscape(fullName))
	return c.doDelete(ctx, path)
}

// DeleteVersion permanently deletes a single version.
// If it was the last version, the package is also deleted.
func (c *Client) DeleteVersion(ctx context.Context, fullName, version string) error {
	path := fmt.Sprintf("/v1/packages/%s/versions/%s", url.PathEscape(fullName), url.PathEscape(version))
	return c.doDelete(ctx, path)
}

// GetMe returns the current user.
func (c *Client) GetMe(ctx context.Context) (*UserInfo, error) {
	var result UserInfo
	if err := c.get(ctx, "/v1/me", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Download fetches the formula archive for a version.
func (c *Client) Download(ctx context.Context, fullName, version string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/v1/packages/%s/versions/%s/archive", url.PathEscape(fullName), url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	return c.doJSON(ctx, "GET", path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	return c.doJSON(ctx, "POST", path, body, result)
}

// --- Org APIs ---

// CreateOrg creates a new organization.
func (c *Client) CreateOrg(ctx context.Context, name, displayName string) (*OrgInfo, error) {
	body := map[string]string{"name": name}
	if displayName != "" {
		body["display_name"] = displayName
	}
	var result OrgInfo
	if err := c.post(ctx, "/v1/orgs", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOrg fetches organization details.
func (c *Client) GetOrg(ctx context.Context, name string) (*OrgDetail, error) {
	var result OrgDetail
	if err := c.get(ctx, "/v1/orgs/"+url.PathEscape(name), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMyOrgs lists orgs the current user belongs to.
func (c *Client) ListMyOrgs(ctx context.Context) ([]OrgInfo, error) {
	var result struct {
		Orgs []OrgInfo `json:"orgs"`
	}
	if err := c.get(ctx, "/v1/orgs", &result); err != nil {
		return nil, err
	}
	return result.Orgs, nil
}

// ListOrgPackages lists packages in an org scope.
func (c *Client) ListOrgPackages(ctx context.Context, name string) ([]PackageInfo, error) {
	var result struct {
		Packages []PackageInfo `json:"packages"`
	}
	if err := c.get(ctx, "/v1/orgs/"+url.PathEscape(name)+"/packages", &result); err != nil {
		return nil, err
	}
	return result.Packages, nil
}

// AddOrgMember adds a member to an org.
func (c *Client) AddOrgMember(ctx context.Context, org, username, role string) error {
	return c.post(ctx, "/v1/orgs/"+url.PathEscape(org)+"/members",
		map[string]string{"username": username, "role": role}, nil)
}

// RemoveOrgMember removes a member from an org.
func (c *Client) RemoveOrgMember(ctx context.Context, org, username string) error {
	return c.doDelete(ctx, "/v1/orgs/"+url.PathEscape(org)+"/members/"+url.PathEscape(username))
}

// DeleteOrg deletes an org (must have 0 packages).
func (c *Client) DeleteOrg(ctx context.Context, name string) error {
	return c.doDelete(ctx, "/v1/orgs/"+url.PathEscape(name))
}

// --- Invitation APIs ---

// InviteOrgMember creates an invitation for a user to join an org.
func (c *Client) InviteOrgMember(ctx context.Context, org, username, role string) (*OrgInvitation, error) {
	var result OrgInvitation
	if err := c.post(ctx, "/v1/orgs/"+url.PathEscape(org)+"/invitations",
		map[string]string{"username": username, "role": role}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOrgInvitations lists invitations for an org.
func (c *Client) ListOrgInvitations(ctx context.Context, org string) ([]OrgInvitation, error) {
	var result struct {
		Invitations []OrgInvitation `json:"invitations"`
	}
	if err := c.get(ctx, "/v1/orgs/"+url.PathEscape(org)+"/invitations", &result); err != nil {
		return nil, err
	}
	return result.Invitations, nil
}

// CancelOrgInvitation cancels a pending invitation.
func (c *Client) CancelOrgInvitation(ctx context.Context, org, invitationID string) error {
	return c.doDelete(ctx, "/v1/orgs/"+url.PathEscape(org)+"/invitations/"+url.PathEscape(invitationID))
}

// ListMyInvitations lists the current user's pending invitations.
func (c *Client) ListMyInvitations(ctx context.Context) ([]OrgInvitation, error) {
	var result struct {
		Invitations []OrgInvitation `json:"invitations"`
	}
	if err := c.get(ctx, "/v1/me/invitations", &result); err != nil {
		return nil, err
	}
	return result.Invitations, nil
}

// AcceptInvitation accepts an org invitation.
func (c *Client) AcceptInvitation(ctx context.Context, invitationID string) error {
	return c.post(ctx, "/v1/me/invitations/"+url.PathEscape(invitationID)+"/accept", nil, nil)
}

// DeclineInvitation declines an org invitation.
func (c *Client) DeclineInvitation(ctx context.Context, invitationID string) error {
	return c.post(ctx, "/v1/me/invitations/"+url.PathEscape(invitationID)+"/decline", nil, nil)
}

// --- Package Access APIs ---

// GetPackageAccess lists users with access to a package.
func (c *Client) GetPackageAccess(ctx context.Context, fullName string) ([]PackageAccessEntry, error) {
	var result struct {
		Access []PackageAccessEntry `json:"access"`
	}
	path := fmt.Sprintf("/v1/packages/%s/access", url.PathEscape(fullName))
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Access, nil
}

// UpdatePackageAccess adds or removes users from a package's access list.
func (c *Client) UpdatePackageAccess(ctx context.Context, fullName string, add, remove []string) error {
	path := fmt.Sprintf("/v1/packages/%s/access", url.PathEscape(fullName))
	body := map[string][]string{}
	if len(add) > 0 {
		body["add"] = add
	}
	if len(remove) > 0 {
		body["remove"] = remove
	}
	return c.doPatch(ctx, path, body)
}

// SetVisibility changes a package's visibility.
func (c *Client) SetVisibility(ctx context.Context, fullName, visibility string) error {
	path := fmt.Sprintf("/v1/packages/%s/visibility", url.PathEscape(fullName))
	return c.doPatch(ctx, path, map[string]string{"visibility": visibility})
}

// --- Dist-tag APIs ---

// ListTags lists dist-tags for a package.
func (c *Client) ListTags(ctx context.Context, fullName string) (map[string]string, error) {
	var result struct {
		Tags map[string]string `json:"tags"`
	}
	if err := c.get(ctx, "/v1/packages/"+url.PathEscape(fullName)+"/tags", &result); err != nil {
		return nil, err
	}
	return result.Tags, nil
}

// SetTag sets a dist-tag.
func (c *Client) SetTag(ctx context.Context, fullName, tag, version string) error {
	path := fmt.Sprintf("/v1/packages/%s/tags/%s", url.PathEscape(fullName), url.PathEscape(tag))
	return c.doPut(ctx, path, map[string]string{"version": version})
}

// DeleteTag removes a dist-tag.
func (c *Client) DeleteTag(ctx context.Context, fullName, tag string) error {
	path := fmt.Sprintf("/v1/packages/%s/tags/%s", url.PathEscape(fullName), url.PathEscape(tag))
	return c.doDelete(ctx, path)
}

// --- Sync APIs ---

// PushSyncProfile uploads the sync profile.
func (c *Client) PushSyncProfile(ctx context.Context, profile *SyncProfile) error {
	return c.doPut(ctx, "/v1/me/sync-profile", profile)
}

// GetSyncProfile downloads the sync profile.
func (c *Client) GetSyncProfile(ctx context.Context) (*SyncProfileResponse, error) {
	var result SyncProfileResponse
	if err := c.get(ctx, "/v1/me/sync-profile", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RecordSyncPull records a pull event.
func (c *Client) RecordSyncPull(ctx context.Context, device string) error {
	return c.post(ctx, "/v1/me/sync-pull", map[string]string{"device": device}, nil)
}

// --- Telemetry ---

// ReportInstall reports an install for telemetry.
func (c *Client) ReportInstall(ctx context.Context, pkg, version string, agents []string, sourceType string) {
	body := map[string]interface{}{
		"package":     pkg,
		"version":     version,
		"agents":      agents,
		"source_type": sourceType,
	}
	_ = c.post(ctx, "/v1/telemetry/install", body, nil)
}

// --- HTTP helpers ---

func (c *Client) doPut(ctx context.Context, path string, body any) error {
	return c.doJSON(ctx, "PUT", path, body, nil)
}

func (c *Client) doPatch(ctx context.Context, path string, body any) error {
	return c.doJSON(ctx, "PATCH", path, body, nil)
}

func (c *Client) doDelete(ctx context.Context, path string) error {
	return c.doJSON(ctx, "DELETE", path, nil, nil)
}

// doJSON is a generic helper for JSON API requests.
func (c *Client) doJSON(ctx context.Context, method, path string, body any, result any) error {
	output.FromContext(ctx).Verbose(ctx, "registry: %s %s%s", method, c.BaseURL, path)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	output.FromContext(ctx).Verbose(ctx, "registry: %s %s%s → %d", method, c.BaseURL, path, resp.StatusCode)

	if resp.StatusCode >= 400 {
		return parseError(resp)
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// APIError represents an error returned by the registry API.
type APIError struct {
	StatusCode int
	Msg        string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Msg)
}

// IsNotFound reports whether the error is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

func parseError(resp *http.Response) error {
	var errResp ErrorResponse
	limitedBody := io.LimitReader(resp.Body, 1024*1024)
	if err := json.NewDecoder(limitedBody).Decode(&errResp); err != nil {
		return &APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	msg := errResp.Error
	if errResp.Message != "" {
		msg = errResp.Message
	}
	return &APIError{StatusCode: resp.StatusCode, Msg: msg}
}

// --- Transfer APIs ---

// InitiateTransfer starts a package transfer to another scope.
func (c *Client) InitiateTransfer(ctx context.Context, fullName, to, message string) (*TransferRequest, error) {
	path := fmt.Sprintf("/v1/packages/%s/transfer", url.PathEscape(fullName))
	body := map[string]string{"to": to}
	if message != "" {
		body["message"] = message
	}
	var result TransferRequest
	if err := c.post(ctx, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelTransfer cancels a pending package transfer.
func (c *Client) CancelTransfer(ctx context.Context, fullName string) error {
	path := fmt.Sprintf("/v1/packages/%s/transfer", url.PathEscape(fullName))
	return c.doDelete(ctx, path)
}

// ListMyTransfers lists incoming transfer requests.
func (c *Client) ListMyTransfers(ctx context.Context) ([]TransferRequest, error) {
	var result struct {
		Transfers []TransferRequest `json:"transfers"`
	}
	if err := c.get(ctx, "/v1/me/transfers", &result); err != nil {
		return nil, err
	}
	return result.Transfers, nil
}

// AcceptTransfer accepts a transfer request.
func (c *Client) AcceptTransfer(ctx context.Context, transferID string) error {
	return c.post(ctx, "/v1/me/transfers/"+url.PathEscape(transferID)+"/accept", nil, nil)
}

// DeclineTransfer declines a transfer request.
func (c *Client) DeclineTransfer(ctx context.Context, transferID string) error {
	return c.post(ctx, "/v1/me/transfers/"+url.PathEscape(transferID)+"/decline", nil, nil)
}

// --- Rename APIs ---

// RenamePackage renames a package within the same scope.
func (c *Client) RenamePackage(ctx context.Context, fullName, newName, confirm string) (*RenameResult, error) {
	path := fmt.Sprintf("/v1/packages/%s/rename", url.PathEscape(fullName))
	var result RenameResult
	if err := c.doJSON(ctx, "PATCH", path, map[string]string{"new_name": newName, "confirm": confirm}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RenameOrg renames an organization.
func (c *Client) RenameOrg(ctx context.Context, orgName, newName, confirm string) (*RenameResult, error) {
	path := fmt.Sprintf("/v1/orgs/%s/rename", url.PathEscape(orgName))
	var result RenameResult
	if err := c.doJSON(ctx, "PATCH", path, map[string]string{"new_name": newName, "confirm": confirm}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RenameUser renames the current user.
func (c *Client) RenameUser(ctx context.Context, newUsername, confirm string) (*RenameResult, error) {
	var result RenameResult
	if err := c.doJSON(ctx, "PATCH", "/v1/me/rename", map[string]string{"new_username": newUsername, "confirm": confirm}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Notification APIs ---

// ListNotifications lists user notifications.
func (c *Client) ListNotifications(ctx context.Context, unreadOnly bool) ([]Notification, error) {
	var result struct {
		Notifications []Notification `json:"notifications"`
	}
	path := "/v1/me/notifications"
	if unreadOnly {
		path += "?unread_only=true"
	}
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Notifications, nil
}

// MarkNotificationRead marks a notification as read.
func (c *Client) MarkNotificationRead(ctx context.Context, id string) error {
	return c.doPatch(ctx, "/v1/me/notifications/"+url.PathEscape(id), map[string]bool{"read": true})
}

// DismissNotification dismisses a notification.
func (c *Client) DismissNotification(ctx context.Context, id string) error {
	return c.doDelete(ctx, "/v1/me/notifications/"+url.PathEscape(id))
}

// --- Org Lifecycle APIs ---

// ArchiveOrg archives an organization (freeze publishing).
func (c *Client) ArchiveOrg(ctx context.Context, name string) error {
	return c.post(ctx, "/v1/orgs/"+url.PathEscape(name)+"/archive", nil, nil)
}

// UnarchiveOrg unarchives an organization.
func (c *Client) UnarchiveOrg(ctx context.Context, name string) error {
	return c.post(ctx, "/v1/orgs/"+url.PathEscape(name)+"/unarchive", nil, nil)
}

// LeaveOrg leaves an organization.
func (c *Client) LeaveOrg(ctx context.Context, name string) error {
	return c.post(ctx, "/v1/orgs/"+url.PathEscape(name)+"/leave", nil, nil)
}

// DissolveOrg dissolves an organization.
func (c *Client) DissolveOrg(ctx context.Context, name, action, transferTo, confirm string) error {
	body := map[string]string{"action": action, "confirm": confirm}
	if transferTo != "" {
		body["transfer_to"] = transferTo
	}
	return c.post(ctx, "/v1/orgs/"+url.PathEscape(name)+"/dissolve", body, nil)
}

// ── Token Management ──

// CreateToken creates a new API token with optional scopes.
func (c *Client) CreateToken(ctx context.Context, req CreateTokenRequest) (*CreateTokenResponse, error) {
	var result CreateTokenResponse
	if err := c.doJSON(ctx, "POST", "/v1/me/tokens", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTokens returns all tokens for the current user.
func (c *Client) ListTokens(ctx context.Context) ([]TokenInfo, error) {
	var result struct {
		Tokens []TokenInfo `json:"tokens"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/me/tokens", nil, &result); err != nil {
		return nil, err
	}
	return result.Tokens, nil
}

// RevokeToken deletes a token by ID.
func (c *Client) RevokeToken(ctx context.Context, tokenID string) error {
	return c.doJSON(ctx, "DELETE", "/v1/me/tokens/"+url.PathEscape(tokenID), nil, nil)
}

// StarPackage stars a package.
func (c *Client) StarPackage(ctx context.Context, fullName string) error {
	return c.doJSON(ctx, "PUT", "/v1/packages/"+url.PathEscape(fullName)+"/star", nil, nil)
}

// UnstarPackage unstars a package.
func (c *Client) UnstarPackage(ctx context.Context, fullName string) error {
	return c.doJSON(ctx, "DELETE", "/v1/packages/"+url.PathEscape(fullName)+"/star", nil, nil)
}

// ListStars returns the user's starred packages.
func (c *Client) ListStars(ctx context.Context) ([]StarEntry, error) {
	var result struct {
		Stars []StarEntry `json:"stars"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/me/stars", nil, &result); err != nil {
		return nil, err
	}
	return result.Stars, nil
}

// CreateStarList creates a new star list.
func (c *Client) CreateStarList(ctx context.Context, name, visibility string) (*StarList, error) {
	var result StarList
	body := map[string]string{"name": name, "visibility": visibility}
	if err := c.doJSON(ctx, "POST", "/v1/me/star-lists", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListStarLists returns the user's star lists.
func (c *Client) ListStarLists(ctx context.Context) ([]StarList, error) {
	var result struct {
		Lists []StarList `json:"lists"`
	}
	if err := c.doJSON(ctx, "GET", "/v1/me/star-lists", nil, &result); err != nil {
		return nil, err
	}
	return result.Lists, nil
}

// GetPublicStarList returns a public star list by user and slug.
func (c *Client) GetPublicStarList(ctx context.Context, username, slug string) ([]StarEntry, error) {
	path := fmt.Sprintf("/v1/users/%s/star-lists/%s", url.PathEscape(username), url.PathEscape(slug))
	var result struct {
		Stars []StarEntry `json:"stars"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Stars, nil
}

// UploadArtifact uploads a platform-specific binary artifact for a version.
// Uses streaming multipart to avoid buffering the entire archive in memory.
func (c *Client) UploadArtifact(ctx context.Context, fullName, version, platform string, archive io.Reader) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write multipart body in a goroutine so the HTTP request can stream it.
	errCh := make(chan error, 1)
	go func() {
		var writeErr error
		defer func() {
			if writeErr != nil {
				pw.CloseWithError(writeErr) // signal reader to abort
			} else {
				pw.Close()
			}
			errCh <- writeErr
		}()

		if err := writer.WriteField("platform", platform); err != nil {
			writeErr = fmt.Errorf("write platform field: %w", err)
			return
		}

		archivePart, err := writer.CreateFormFile("archive", "artifact.tar.gz")
		if err != nil {
			writeErr = fmt.Errorf("create archive part: %w", err)
			return
		}
		if _, err := io.Copy(archivePart, archive); err != nil {
			writeErr = fmt.Errorf("write archive: %w", err)
			return
		}

		writeErr = writer.Close()
	}()

	apiPath := fmt.Sprintf("/v1/packages/%s/versions/%s/artifacts", url.PathEscape(fullName), url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+apiPath, pr)
	if err != nil {
		pr.Close()  // unblock writer goroutine
		<-errCh     // wait for goroutine to exit
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", config.UserAgent())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		pr.Close()  // unblock writer goroutine
		<-errCh     // wait for goroutine to exit
		return fmt.Errorf("upload artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for errors from the multipart writer goroutine.
	if writeErr := <-errCh; writeErr != nil {
		return writeErr
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return parseError(resp)
	}
	return nil
}

// ListArtifacts returns all platform artifacts for a version.
func (c *Client) ListArtifacts(ctx context.Context, fullName, version string) ([]ArtifactInfo, error) {
	var result struct {
		Artifacts []ArtifactInfo `json:"artifacts"`
	}
	path := fmt.Sprintf("/v1/packages/%s/versions/%s/artifacts", url.PathEscape(fullName), url.PathEscape(version))
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.Artifacts, nil
}

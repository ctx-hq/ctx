package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
)

// Client communicates with the getctx.org API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New creates a new registry client.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: http.DefaultClient,
	}
}

// Search searches packages.
func (c *Client) Search(ctx context.Context, query string, pkgType string, platform string, limit int) (*SearchResult, error) {
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
	if err := c.post(ctx, "/v1/resolve", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Publish publishes a package (manifest + optional archive).
func (c *Client) Publish(ctx context.Context, manifestData []byte, archive io.Reader) (*PublishResponse, error) {
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
	path := fmt.Sprintf("/v1/packages/%s/versions/%s", url.PathEscape(fullName), url.PathEscape(version))
	return c.doPatch(ctx, path, map[string]bool{"yanked": true})
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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/publish", body)
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
	path := fmt.Sprintf("/v1/yank/%s/%s", url.PathEscape(fullName), url.PathEscape(version))
	return c.post(ctx, path, nil, nil)
}

// GetMe returns the current user.
func (c *Client) GetMe(ctx context.Context) (*UserInfo, error) {
	var result UserInfo
	if err := c.get(ctx, "/v1/users/me", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Download fetches the formula archive for a version.
func (c *Client) Download(ctx context.Context, fullName, version string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/v1/download/%s/%s", url.PathEscape(fullName), url.PathEscape(version))
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
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bodyReader)
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

func parseError(resp *http.Response) error {
	var errResp ErrorResponse
	limitedBody := io.LimitReader(resp.Body, 1024*1024) // 1MB max for error responses
	if err := json.NewDecoder(limitedBody).Decode(&errResp); err != nil {
		return fmt.Errorf("API error (status %d)", resp.StatusCode)
	}
	msg := errResp.Error
	if errResp.Message != "" {
		msg = errResp.Message
	}
	return fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
}

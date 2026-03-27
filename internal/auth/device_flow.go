package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowResponse is returned when starting the device flow.
type DeviceFlowResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the final auth token.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

// StartDeviceFlow initiates the GitHub OAuth device flow via our API.
func StartDeviceFlow(ctx context.Context, registryURL string) (*DeviceFlowResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", registryURL+"/v1/auth/device", nil)
	if err != nil {
		return nil, fmt.Errorf("create device flow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("start device flow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device flow returned status %d", resp.StatusCode)
	}

	var result DeviceFlowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode device flow response: %w", err)
	}
	return &result, nil
}

// PollForToken polls until the user completes authorization.
func PollForToken(ctx context.Context, registryURL, deviceCode string, interval int) (*TokenResponse, error) {
	if interval < 1 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := checkToken(ctx, registryURL, deviceCode)
			if err != nil {
				return nil, err
			}
			if token.Error == "authorization_pending" {
				continue
			}
			if token.Error == "slow_down" {
				interval += 5
				ticker.Reset(time.Duration(interval) * time.Second)
				continue
			}
			if token.Error != "" {
				return nil, fmt.Errorf("auth error: %s", token.Error)
			}
			return token, nil
		}
	}
}

func checkToken(ctx context.Context, registryURL, deviceCode string) (*TokenResponse, error) {
	form := url.Values{"device_code": {deviceCode}}
	req, err := http.NewRequestWithContext(ctx, "POST", registryURL+"/v1/auth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll token: %w", err)
	}
	defer resp.Body.Close()

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &result, nil
}

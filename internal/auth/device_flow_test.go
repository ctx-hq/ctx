package auth

import (
	"encoding/json"
	"testing"
)

func TestDeviceFlowResponseUnmarshal(t *testing.T) {
	raw := `{
		"device_code": "dc123",
		"user_code": "ABCD1234",
		"verification_uri": "https://getctx.org/login/device",
		"verification_uri_complete": "https://getctx.org/login/device?code=ABCD1234",
		"expires_in": 900,
		"interval": 5
	}`

	var resp DeviceFlowResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.DeviceCode != "dc123" {
		t.Errorf("DeviceCode = %q, want %q", resp.DeviceCode, "dc123")
	}
	if resp.UserCode != "ABCD1234" {
		t.Errorf("UserCode = %q, want %q", resp.UserCode, "ABCD1234")
	}
	if resp.VerificationURI != "https://getctx.org/login/device" {
		t.Errorf("VerificationURI = %q", resp.VerificationURI)
	}
	if resp.VerificationURIComplete != "https://getctx.org/login/device?code=ABCD1234" {
		t.Errorf("VerificationURIComplete = %q", resp.VerificationURIComplete)
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", resp.ExpiresIn)
	}
	if resp.Interval != 5 {
		t.Errorf("Interval = %d, want 5", resp.Interval)
	}
}

func TestDeviceFlowResponseUnmarshal_BackwardCompat(t *testing.T) {
	// Old API response without verification_uri_complete
	raw := `{
		"device_code": "dc456",
		"user_code": "XYZ",
		"verification_uri": "https://getctx.org/login/device",
		"expires_in": 900,
		"interval": 5
	}`

	var resp DeviceFlowResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.VerificationURIComplete != "" {
		t.Errorf("VerificationURIComplete should be empty, got %q", resp.VerificationURIComplete)
	}
}

func TestBrowserURL_PrefersComplete(t *testing.T) {
	resp := &DeviceFlowResponse{
		VerificationURI:         "https://getctx.org/login/device",
		VerificationURIComplete: "https://getctx.org/login/device?code=ABC",
	}

	got := resp.BrowserURL()
	if got != "https://getctx.org/login/device?code=ABC" {
		t.Errorf("BrowserURL() = %q, want complete URI", got)
	}
}

func TestBrowserURL_FallsBackToBase(t *testing.T) {
	resp := &DeviceFlowResponse{
		VerificationURI:         "https://getctx.org/login/device",
		VerificationURIComplete: "",
	}

	got := resp.BrowserURL()
	if got != "https://getctx.org/login/device" {
		t.Errorf("BrowserURL() = %q, want base URI", got)
	}
}

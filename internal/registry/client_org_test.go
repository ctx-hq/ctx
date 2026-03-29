package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOrg_SendsCorrectPayload(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/orgs" {
			t.Errorf("path = %q, want /v1/orgs", r.URL.Path)
		}

		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(OrgInfo{
			ID:   "org-123",
			Name: "myteam",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	result, err := c.CreateOrg(context.Background(), "myteam", "My Team")
	if err != nil {
		t.Fatalf("CreateOrg error: %v", err)
	}

	if gotBody["name"] != "myteam" {
		t.Errorf("body.name = %q, want %q", gotBody["name"], "myteam")
	}
	if gotBody["display_name"] != "My Team" {
		t.Errorf("body.display_name = %q, want %q", gotBody["display_name"], "My Team")
	}
	if result.Name != "myteam" {
		t.Errorf("result.Name = %q, want %q", result.Name, "myteam")
	}
}

func TestGetOrg_ReturnsDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(OrgDetail{
			OrgInfo:  OrgInfo{Name: "testorg"},
			Members:  5,
			Packages: 12,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	detail, err := c.GetOrg(context.Background(), "testorg")
	if err != nil {
		t.Fatalf("GetOrg error: %v", err)
	}
	if detail.Members != 5 {
		t.Errorf("Members = %d, want 5", detail.Members)
	}
	if detail.Packages != 12 {
		t.Errorf("Packages = %d, want 12", detail.Packages)
	}
}

func TestAddOrgMember_SendsRole(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.AddOrgMember(context.Background(), "myorg", "alice", "admin")
	if err != nil {
		t.Fatalf("AddOrgMember error: %v", err)
	}

	if gotBody["username"] != "alice" {
		t.Errorf("username = %q, want %q", gotBody["username"], "alice")
	}
	if gotBody["role"] != "admin" {
		t.Errorf("role = %q, want %q", gotBody["role"], "admin")
	}
}

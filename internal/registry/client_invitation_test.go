package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInviteOrgMember_SendsCorrectPayload(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(OrgInvitation{
			ID:      "inv-1",
			OrgName: "myorg",
			Invitee: "alice",
			Role:    "admin",
			Status:  "pending",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	inv, err := c.InviteOrgMember(context.Background(), "myorg", "alice", "admin")
	if err != nil {
		t.Fatalf("InviteOrgMember error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/orgs/myorg/invitations" {
		t.Errorf("path = %q, want /v1/orgs/myorg/invitations", gotPath)
	}
	if gotBody["username"] != "alice" {
		t.Errorf("body.username = %q, want %q", gotBody["username"], "alice")
	}
	if gotBody["role"] != "admin" {
		t.Errorf("body.role = %q, want %q", gotBody["role"], "admin")
	}
	if inv.ID != "inv-1" {
		t.Errorf("result.ID = %q, want %q", inv.ID, "inv-1")
	}
}

func TestListOrgInvitations_ReturnsInvitations(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"invitations": []OrgInvitation{
				{ID: "inv-1", OrgName: "myorg", Invitee: "alice", Status: "pending"},
				{ID: "inv-2", OrgName: "myorg", Invitee: "bob", Status: "pending"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	invs, err := c.ListOrgInvitations(context.Background(), "myorg")
	if err != nil {
		t.Fatalf("ListOrgInvitations error: %v", err)
	}

	if gotPath != "/v1/orgs/myorg/invitations" {
		t.Errorf("path = %q, want /v1/orgs/myorg/invitations", gotPath)
	}
	if len(invs) != 2 {
		t.Fatalf("got %d invitations, want 2", len(invs))
	}
}

func TestCancelOrgInvitation_SendsDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.CancelOrgInvitation(context.Background(), "myorg", "inv-1")
	if err != nil {
		t.Fatalf("CancelOrgInvitation error: %v", err)
	}

	if gotMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v1/orgs/myorg/invitations/inv-1" {
		t.Errorf("path = %q, want /v1/orgs/myorg/invitations/inv-1", gotPath)
	}
}

func TestListMyInvitations_ReturnsInvitations(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"invitations": []OrgInvitation{
				{ID: "inv-1", OrgName: "teamx", Status: "pending"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	invs, err := c.ListMyInvitations(context.Background())
	if err != nil {
		t.Fatalf("ListMyInvitations error: %v", err)
	}

	if gotPath != "/v1/me/invitations" {
		t.Errorf("path = %q, want /v1/me/invitations", gotPath)
	}
	if len(invs) != 1 {
		t.Fatalf("got %d invitations, want 1", len(invs))
	}
}

func TestAcceptInvitation_SendsPost(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.AcceptInvitation(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("AcceptInvitation error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/me/invitations/inv-1/accept" {
		t.Errorf("path = %q, want /v1/me/invitations/inv-1/accept", gotPath)
	}
}

func TestDeclineInvitation_SendsPost(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.DeclineInvitation(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("DeclineInvitation error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/me/invitations/inv-1/decline" {
		t.Errorf("path = %q, want /v1/me/invitations/inv-1/decline", gotPath)
	}
}

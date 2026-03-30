package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPackageAccess_ReturnsEntries(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access": []PackageAccessEntry{
				{Username: "alice", GrantedBy: "owner1"},
				{Username: "bob", GrantedBy: "owner1"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	entries, err := c.GetPackageAccess(context.Background(), "@scope/pkg")
	if err != nil {
		t.Fatalf("GetPackageAccess error: %v", err)
	}

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if want := "/v1/packages/@scope/pkg/access"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Username != "alice" {
		t.Errorf("entries[0].Username = %q, want %q", entries[0].Username, "alice")
	}
}

func TestUpdatePackageAccess_SendsAddAndRemove(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.UpdatePackageAccess(context.Background(), "@scope/pkg", []string{"alice"}, []string{"bob"})
	if err != nil {
		t.Fatalf("UpdatePackageAccess error: %v", err)
	}

	if gotMethod != "PATCH" {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if want := "/v1/packages/@scope/pkg/access"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if len(gotBody["add"]) != 1 || gotBody["add"][0] != "alice" {
		t.Errorf("body.add = %v, want [alice]", gotBody["add"])
	}
	if len(gotBody["remove"]) != 1 || gotBody["remove"][0] != "bob" {
		t.Errorf("body.remove = %v, want [bob]", gotBody["remove"])
	}
}

func TestUpdatePackageAccess_OmitsEmptyFields(t *testing.T) {
	var gotBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.UpdatePackageAccess(context.Background(), "@scope/pkg", []string{"alice"}, nil)
	if err != nil {
		t.Fatalf("UpdatePackageAccess error: %v", err)
	}

	if _, hasRemove := gotBody["remove"]; hasRemove {
		t.Error("body should not contain 'remove' when remove list is nil")
	}
	if _, hasAdd := gotBody["add"]; !hasAdd {
		t.Error("body should contain 'add'")
	}
}

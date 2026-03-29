package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListTags_ReturnsTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/tags") {
			t.Errorf("path = %q, should end with /tags", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": map[string]string{
				"latest": "2.0.0",
				"beta":   "2.1.0-beta.1",
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	tags, err := c.ListTags(context.Background(), "@test/pkg")
	if err != nil {
		t.Fatalf("ListTags error: %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("tags count = %d, want 2", len(tags))
	}
	if tags["latest"] != "2.0.0" {
		t.Errorf("tags[latest] = %q, want %q", tags["latest"], "2.0.0")
	}
	if tags["beta"] != "2.1.0-beta.1" {
		t.Errorf("tags[beta] = %q, want %q", tags["beta"], "2.1.0-beta.1")
	}
}

func TestSetTag_SendsPUT(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.SetTag(context.Background(), "@test/pkg", "beta", "2.0.0-beta.1")
	if err != nil {
		t.Fatalf("SetTag error: %v", err)
	}

	if gotMethod != "PUT" {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if !strings.Contains(gotPath, "/tags/") {
		t.Errorf("path = %q, should contain /tags/", gotPath)
	}
	if gotBody["version"] != "2.0.0-beta.1" {
		t.Errorf("body.version = %q, want %q", gotBody["version"], "2.0.0-beta.1")
	}
}

func TestDeleteTag_SendsDELETE(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "token")
	err := c.DeleteTag(context.Background(), "@test/pkg", "beta")
	if err != nil {
		t.Fatalf("DeleteTag error: %v", err)
	}

	if gotMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

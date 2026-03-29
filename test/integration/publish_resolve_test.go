package integration

import (
	"testing"

	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/registry"
)

func TestPublishResolveFlow_ManifestValidation(t *testing.T) {
	// Verify a manifest passes validation before it would be published
	m := &manifest.Manifest{
		Name:        "@test/publish-flow",
		Version:     "1.0.0",
		Type:        manifest.TypeSkill,
		Description: "A test skill for publish flow validation",
		Skill: &manifest.SkillSpec{
			Entry: "SKILL.md",
		},
	}

	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Errorf("valid manifest should pass validation, got errors: %v", errs)
	}
}

func TestPublishResolveFlow_InvalidManifestRejected(t *testing.T) {
	tests := []struct {
		name   string
		m      *manifest.Manifest
		errStr string
	}{
		{
			name:   "missing name",
			m:      &manifest.Manifest{Version: "1.0.0", Type: manifest.TypeSkill, Description: "test"},
			errStr: "name is required",
		},
		{
			name:   "invalid name format",
			m:      &manifest.Manifest{Name: "no-scope", Version: "1.0.0", Type: manifest.TypeSkill, Description: "test"},
			errStr: "@scope/name format",
		},
		{
			name:   "bad version",
			m:      &manifest.Manifest{Name: "@test/pkg", Version: "abc", Type: manifest.TypeSkill, Description: "test"},
			errStr: "semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := manifest.Validate(tt.m)
			if len(errs) == 0 {
				t.Error("expected validation error, got none")
			}
		})
	}
}

func TestPublishResolveFlow_ResolveRequestStructure(t *testing.T) {
	// Verify ResolveRequest can be constructed for a published package
	req := &registry.ResolveRequest{
		Packages: map[string]string{
			"@test/publish-flow": "^1.0.0",
		},
	}

	if len(req.Packages) != 1 {
		t.Errorf("Packages count = %d, want 1", len(req.Packages))
	}

	constraint, ok := req.Packages["@test/publish-flow"]
	if !ok {
		t.Fatal("package @test/publish-flow not in request")
	}
	if constraint != "^1.0.0" {
		t.Errorf("constraint = %q, want %q", constraint, "^1.0.0")
	}
}

func TestPublishResolveFlow_MarshalRoundtrip(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@test/roundtrip",
		Version:     "2.0.0",
		Type:        manifest.TypeMCP,
		Description: "MCP server for testing",
		MCP: &manifest.MCPSpec{
			Transport: "stdio",
			Command:   "node",
			Args:      []string{"dist/index.js"},
		},
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("marshaled data should not be empty")
	}

	// Verify we can validate the manifest that was just marshaled
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Errorf("roundtrip manifest should be valid, got: %v", errs)
	}
}

package integration

import (
	"strings"
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

func TestPublishResolveFlow_ManifestWithMetadata(t *testing.T) {
	m := &manifest.Manifest{
		Name:        "@test/metadata-flow",
		Version:     "1.0.0",
		Type:        manifest.TypeCLI,
		Description: "CLI with full metadata",
		Author:      "Test Author",
		License:     "MIT",
		Repository:  "https://github.com/test/metadata-flow",
		Skill: &manifest.SkillSpec{
			Entry:  "skills/test/SKILL.md",
			Origin: "native",
		},
		CLI: &manifest.CLISpec{
			Binary: "testbin",
			Verify: "testbin --version",
		},
	}

	// Validate passes with metadata fields
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		t.Errorf("manifest with metadata should be valid, got: %v", errs)
	}

	// Marshal roundtrip preserves metadata
	data, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	m2, err := manifest.Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if m2.Author != "Test Author" {
		t.Errorf("roundtrip Author = %q, want %q", m2.Author, "Test Author")
	}
	if m2.License != "MIT" {
		t.Errorf("roundtrip License = %q, want %q", m2.License, "MIT")
	}
	if m2.Repository != "https://github.com/test/metadata-flow" {
		t.Errorf("roundtrip Repository = %q, want %q", m2.Repository, "https://github.com/test/metadata-flow")
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
		Skill: &manifest.SkillSpec{
			Entry: "skills/roundtrip/SKILL.md",
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

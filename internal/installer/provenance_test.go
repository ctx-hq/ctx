package installer

import (
	"testing"
)

func TestNewProvenance(t *testing.T) {
	p := NewProvenance("registry", "https://api.getctx.org/v1/download/...", "^1.0", "1.5.0")
	if p.Source != "registry" {
		t.Errorf("source = %q, want registry", p.Source)
	}
	if p.InstalledBy != "ctx@1.5.0" {
		t.Errorf("installed_by = %q, want ctx@1.5.0", p.InstalledBy)
	}
	if p.InstalledAt == "" {
		t.Error("installed_at should be set")
	}
}

func TestProvenance_IsSyncable(t *testing.T) {
	tests := []struct {
		source   string
		syncable bool
	}{
		{"registry", true},
		{"github", true},
		{"push", true},
		{"local", false},
		{"import:clawhub", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		p := &Provenance{Source: tt.source}
		if got := p.IsSyncable(); got != tt.syncable {
			t.Errorf("IsSyncable(%q) = %v, want %v", tt.source, got, tt.syncable)
		}
	}
}

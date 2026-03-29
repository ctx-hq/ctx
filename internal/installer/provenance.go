package installer

import "time"

// Provenance records the source and install context of a package.
type Provenance struct {
	Source      string   `json:"source"`                 // registry, github, push, local, import:*
	SourceURL   string   `json:"source_url,omitempty"`   // download URL or github ref
	InstalledAt string   `json:"installed_at"`           // ISO timestamp
	InstalledBy string   `json:"installed_by,omitempty"` // ctx version
	Constraint  string   `json:"constraint,omitempty"`   // version constraint used
	OriginalRef string   `json:"original_ref,omitempty"` // e.g. github:user/repo@main
	ImportFrom  string   `json:"import_from,omitempty"`  // e.g. clawhub
	Agents      []string `json:"agents,omitempty"`       // agents linked to
}

// NewProvenance creates a provenance record for a registry install.
func NewProvenance(source, sourceURL, constraint, ctxVersion string) *Provenance {
	return &Provenance{
		Source:      source,
		SourceURL:   sourceURL,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		InstalledBy: "ctx@" + ctxVersion,
		Constraint:  constraint,
	}
}

// IsSyncable returns whether this package can be restored on another device.
func (p *Provenance) IsSyncable() bool {
	return p.Source == "registry" || p.Source == "github" || p.Source == "push"
}

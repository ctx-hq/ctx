// Package app provides the root TUI model for ctx.
package app

import (
	"github.com/ctx-hq/ctx/internal/doctor"
	"github.com/ctx-hq/ctx/internal/installer"
	"github.com/ctx-hq/ctx/internal/registry"
)

// installedLoadedMsg is sent when installed packages have been scanned.
type installedLoadedMsg struct {
	Pkgs []installer.InstalledPackage
	Err  error
}

// searchResultMsg is sent when a search completes.
type searchResultMsg struct {
	Result *registry.SearchResult
	Err    error
}

// packageDetailMsg is sent when package detail has been fetched.
type packageDetailMsg struct {
	Detail *registry.PackageDetail
	Err    error
}

// agentsDetectedMsg is sent when agent detection completes.
type agentsDetectedMsg struct {
	Agents []agentInfo
}

// agentInfo holds summary information about a detected agent.
type agentInfo struct {
	Name       string
	SkillsDir  string
	SkillCount int
}

// doctorResultMsg is sent when doctor checks complete.
type doctorResultMsg struct {
	Result *doctor.Result
}

// filesLoadedMsg is sent when package files have been listed.
type filesLoadedMsg struct {
	Files []FileInfo
	Err   error
}

// fileContentMsg is sent when a file's content has been read.
type fileContentMsg struct {
	Name    string
	Content string
	Err     error
}

// renderedContentMsg is sent when file content has been rendered (glamour/syntax highlight).
type renderedContentMsg struct {
	Key     string // cache key: "dir:filename"
	Content string
}

// statusMsg sets the status bar text.
type statusMsg struct {
	Text string
}

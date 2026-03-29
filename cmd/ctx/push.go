package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/manifest"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/prompt"
	"github.com/ctx-hq/ctx/internal/pushstate"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/ctx-hq/ctx/internal/staging"
	"github.com/spf13/cobra"
)

// Push-specific flags.
var (
	flagPushAll    bool
	flagPushStatus bool
	flagPushDryRun bool
)

func init() {
	pushCmd.Flags().BoolVar(&flagPushAll, "all", false, "Push all modified skills without confirmation")
	pushCmd.Flags().BoolVar(&flagPushStatus, "status", false, "Show push status of all skills")
	pushCmd.Flags().BoolVar(&flagPushDryRun, "dry-run", false, "Preview what would be pushed without publishing")
}

// retryConfig controls retry behavior for publishing.
type retryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// defaultRetryConfig is the production retry configuration.
var defaultRetryConfig = retryConfig{
	MaxRetries:     3,
	InitialBackoff: 500 * time.Millisecond,
	MaxBackoff:     10 * time.Second,
	BackoffFactor:  2.0,
}

const maxParallelPush = 4

// pushMode describes how the push command was invoked.
type pushMode int

const (
	pushModeScan pushMode = iota // no args → scan ~/.ctx/skills/
	pushModeDir                  // explicit directory
	pushModeName                 // resolve by skill name
	pushModeFile                 // single .md file
)

// skillEntry is a discovered skill in ~/.ctx/skills/.
type skillEntry struct {
	FullName string
	Dir      string
	Version  string
	Hash     string
	Dirty    bool
	Error    string // validation/hash error, if any
}

// pushResult is the outcome of pushing one skill.
type pushResult struct {
	FullName string `json:"full_name"`
	Version  string `json:"version"`
	Pushed   bool   `json:"pushed"`
	Error    string `json:"error,omitempty"`
}

var pushCmd = &cobra.Command{
	Use:   "push [name|path]",
	Short: "Push skills to the registry as private packages",
	Long: `Push skills to the registry as private, mutable packages.

Without arguments, scans ~/.ctx/skills/ for modified or unpushed skills
and offers to push them interactively.

With a name argument, resolves the skill from ~/.ctx/skills/ by short name.
With a path argument, pushes the specified directory or .md file.

Examples:
  ctx push                    Scan and push modified skills (interactive)
  ctx push gc                 Push skill by name
  ctx push --all              Push all modified skills without confirmation
  ctx push --status           Show push status of all skills
  ctx push --dry-run          Preview what would be pushed
  ctx push .                  Push current directory
  ctx push ./my-skill         Push a specific directory
  ctx push gc.md              Push a single .md file
  ctx push gc.md --bump patch Bump version and push`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// --status: show status and exit (works offline).
		if flagPushStatus {
			return showPushStatus(w)
		}

		// All other modes require network and auth.
		if err := requireOnline(); err != nil {
			return err
		}
		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in — run 'ctx login' first")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		ps, psErr := pushstate.Load()
		if psErr != nil {
			return psErr
		}

		// Resolve what to push.
		mode, dir := resolveInput(args)

		switch mode {
		case pushModeFile:
			return pushSingleFile(cmd, args[0], w, singleFileOpts{
				defaultVisibility: "private",
				mutable:           true,
				versionBump:       flagBump,
				skipConfirm:       flagYes,
				dryRun:            flagPushDryRun,
			})

		case pushModeDir:
			return pushDirectory(cmd, dir, w, cfg, ps, token)

		case pushModeName:
			resolved, resolveErr := resolveSkillByName(args[0], cfg)
			if resolveErr != nil {
				return resolveErr
			}
			return pushDirectory(cmd, resolved, w, cfg, ps, token)

		case pushModeScan:
			return pushScan(cmd, w, cfg, ps, token)
		}

		return nil
	},
}

// resolveInput determines push mode and target directory from arguments.
func resolveInput(args []string) (pushMode, string) {
	if len(args) == 0 {
		// No args: check if cwd is a skill directory.
		if _, err := os.Stat(manifest.FileName); err == nil {
			return pushModeDir, "."
		}
		return pushModeScan, ""
	}

	arg := args[0]

	// Single .md file → file mode.
	if isSingleFile(arg) {
		return pushModeFile, ""
	}

	// Explicit directory path (., ./, /).
	// Note: "scope/name" style args are handled by resolveSkillByName, not here.
	if arg == "." || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "/") {
		return pushModeDir, arg
	}

	// Check if arg is a directory that exists on disk.
	if info, err := os.Stat(arg); err == nil && info.IsDir() {
		return pushModeDir, arg
	}

	// Otherwise, treat as a skill name to resolve.
	return pushModeName, ""
}

// resolveSkillByName finds a skill in ~/.ctx/skills/ by short name.
func resolveSkillByName(name string, cfg *config.Config) (string, error) {
	skillsDir := config.SkillsDir()
	name = strings.TrimPrefix(name, "@")

	// If name contains /, treat as @scope/name.
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		dir := filepath.Join(skillsDir, parts[0], parts[1])
		if _, err := os.Stat(filepath.Join(dir, manifest.FileName)); err == nil {
			return dir, nil
		}
		return "", output.ErrNotFound("skill", "@"+name)
	}

	// Search all scopes for matching name.
	scopes, err := os.ReadDir(skillsDir)
	if err != nil {
		return "", output.ErrUsageHint(
			"no skills directory found",
			"Run 'ctx init' to create a skill first",
		)
	}

	var matches []string
	var matchDirs []string
	for _, scope := range scopes {
		if !scope.IsDir() {
			continue
		}
		dir := filepath.Join(skillsDir, scope.Name(), name)
		if _, statErr := os.Stat(filepath.Join(dir, manifest.FileName)); statErr == nil {
			matches = append(matches, "@"+scope.Name()+"/"+name)
			matchDirs = append(matchDirs, dir)
		}
	}

	switch len(matches) {
	case 0:
		// Also try matching by logged-in username scope first.
		if cfg.Username != "" {
			dir := filepath.Join(skillsDir, cfg.Username, name)
			if _, statErr := os.Stat(filepath.Join(dir, manifest.FileName)); statErr == nil {
				return dir, nil
			}
		}
		return "", output.ErrNotFound("skill", name)
	case 1:
		return matchDirs[0], nil
	default:
		return "", output.ErrAmbiguous("skill", matches)
	}
}

// scanSkills walks ~/.ctx/skills/ and returns all skills with push status.
func scanSkills(ps *pushstate.PushState) ([]skillEntry, error) {
	skillsDir := config.SkillsDir()
	scopes, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var skills []skillEntry
	for _, scope := range scopes {
		if !scope.IsDir() || strings.HasPrefix(scope.Name(), ".") {
			continue
		}
		scopeDir := filepath.Join(skillsDir, scope.Name())
		names, readErr := os.ReadDir(scopeDir)
		if readErr != nil {
			continue
		}
		for _, name := range names {
			if !name.IsDir() || strings.HasPrefix(name.Name(), ".") {
				continue
			}
			dir := filepath.Join(scopeDir, name.Name())
			entry := skillEntry{
				FullName: "@" + scope.Name() + "/" + name.Name(),
				Dir:      dir,
			}

			// Load manifest for version.
			m, loadErr := manifest.LoadFromDir(dir)
			if loadErr != nil {
				entry.Error = loadErr.Error()
				skills = append(skills, entry)
				continue
			}
			entry.Version = m.Version

			// Validate.
			if errs := manifest.Validate(m); len(errs) > 0 {
				entry.Error = errs[0]
				skills = append(skills, entry)
				continue
			}

			// Compute content hash.
			hash, hashErr := pushstate.HashDir(dir)
			if hashErr != nil {
				entry.Error = hashErr.Error()
				skills = append(skills, entry)
				continue
			}
			entry.Hash = hash
			entry.Dirty = ps.IsDirty(entry.FullName, hash)

			skills = append(skills, entry)
		}
	}
	return skills, nil
}

// showPushStatus displays the push status of all skills.
func showPushStatus(w *output.Writer) error {
	ps, psErr := pushstate.Load()
	if psErr != nil {
		return psErr
	}
	skills, err := scanSkills(ps)
	if err != nil {
		return err
	}
	if len(skills) == 0 {
		return w.OK([]any{},
			output.WithSummary("No skills found. Run 'ctx init' to create one."),
		)
	}

	output.Header("Push Status")

	type statusRow struct {
		name       string
		version    string
		statusText string // plain text for alignment
		statusDisp string // colored text for display
		lastPushed string
	}

	var rows []statusRow
	for _, s := range skills {
		var statusText, statusDisp string
		if s.Error != "" {
			statusText = "error"
			statusDisp = output.Red + statusText + output.Reset
		} else if s.Dirty {
			if _, ok := ps.Skills[s.FullName]; ok {
				statusText = "modified"
				statusDisp = output.Yellow + statusText + output.Reset
			} else {
				statusText = "never pushed"
				statusDisp = output.Cyan + statusText + output.Reset
			}
		} else {
			statusText = "up to date"
			statusDisp = output.Green + statusText + output.Reset
		}
		lastPushed := "—"
		if st, ok := ps.Skills[s.FullName]; ok {
			lastPushed = st.LastPushedAt.Format("2006-01-02 15:04")
		}
		rows = append(rows, statusRow{s.FullName, s.Version, statusText, statusDisp, lastPushed})
	}

	// Print header.
	fmt.Fprintf(os.Stderr, "  %-30s %-10s %-14s %s\n",
		output.Dim+"SKILL"+output.Reset,
		output.Dim+"VERSION"+output.Reset,
		output.Dim+"STATUS"+output.Reset,
		output.Dim+"LAST PUSHED"+output.Reset,
	)
	for _, r := range rows {
		// Pad based on plain text length to avoid ANSI escape code misalignment.
		padding := 14 - len(r.statusText)
		if padding < 0 {
			padding = 0
		}
		fmt.Fprintf(os.Stderr, "  %-30s %-10s %s%s %s\n", r.name, r.version, r.statusDisp, strings.Repeat(" ", padding), r.lastPushed)
	}
	fmt.Fprintln(os.Stderr)

	// Count stats.
	dirty, clean, errCount := 0, 0, 0
	for _, s := range skills {
		switch {
		case s.Error != "":
			errCount++
		case s.Dirty:
			dirty++
		default:
			clean++
		}
	}

	type statusData struct {
		Total  int `json:"total"`
		Dirty  int `json:"dirty"`
		Clean  int `json:"clean"`
		Errors int `json:"errors"`
	}
	return w.OK(statusData{Total: len(skills), Dirty: dirty, Clean: clean, Errors: errCount},
		output.WithSummary(fmt.Sprintf("%d skills: %d modified, %d up to date, %d errors", len(skills), dirty, clean, errCount)),
	)
}

// pushScan handles the no-args scan mode.
func pushScan(cmd *cobra.Command, w *output.Writer, cfg *config.Config, ps *pushstate.PushState, token string) error {
	output.Info("Scanning %s...", config.SkillsDir())

	skills, err := scanSkills(ps)
	if err != nil {
		return err
	}
	if len(skills) == 0 {
		return w.OK([]any{},
			output.WithSummary("No skills found. Run 'ctx init' to create one."),
		)
	}

	// Filter to dirty (pushable) skills.
	var dirty []skillEntry
	for _, s := range skills {
		if s.Error != "" {
			output.Warn("%s: %s (skipped)", s.FullName, s.Error)
			continue
		}
		if s.Dirty {
			dirty = append(dirty, s)
		}
	}

	if len(dirty) == 0 {
		return w.OK([]any{},
			output.WithSummary("All skills up to date."),
		)
	}

	// Display dirty skills.
	fmt.Fprintln(os.Stderr)
	for _, s := range dirty {
		status := output.Yellow + "modified" + output.Reset
		if _, ok := ps.Skills[s.FullName]; !ok {
			status = output.Cyan + "never pushed" + output.Reset
		}
		fmt.Fprintf(os.Stderr, "  %s %s (%s)\n", s.FullName, output.Dim+s.Version+output.Reset, status)
	}
	fmt.Fprintln(os.Stderr)

	// Dry-run: show and exit.
	if flagPushDryRun {
		return w.OK(dirty,
			output.WithSummary(fmt.Sprintf("Would push %d skill(s).", len(dirty))),
		)
	}

	// Confirm unless --all or --yes.
	if !flagPushAll && !flagYes {
		p := prompt.DefaultPrompter()
		confirmed, confirmErr := p.Confirm(fmt.Sprintf("Push %d skill(s)?", len(dirty)), true)
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			output.Info("Cancelled.")
			return nil
		}
	}

	// Batch push.
	reg := registry.New(cfg.RegistryURL(), token)
	return pushBatch(cmd.Context(), dirty, reg, ps, w)
}

// pushBatch pushes multiple skills concurrently.
func pushBatch(ctx context.Context, skills []skillEntry, reg *registry.Client, ps *pushstate.PushState, w *output.Writer) error {
	results := make([]pushResult, len(skills))
	sem := make(chan struct{}, maxParallelPush)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for idx, skill := range skills {
		wg.Add(1)
		go func(i int, s skillEntry) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			// Check for cancellation.
			if ctx.Err() != nil {
				results[i] = pushResult{FullName: s.FullName, Error: "cancelled"}
				return
			}

			res, err := pushOneSkill(ctx, s, reg)
			if err != nil {
				results[i] = pushResult{FullName: s.FullName, Error: err.Error()}
				output.Error("%s: %v", s.FullName, err)
				return
			}

			results[i] = pushResult{FullName: s.FullName, Version: res.Version, Pushed: true}
			output.Success("%s@%s", res.FullName, res.Version)

			mu.Lock()
			ps.RecordPush(s.FullName, s.Hash, res.Version, s.Dir)
			mu.Unlock()
		}(idx, skill)
	}
	wg.Wait()

	// Save state once after all pushes (preserves partial success).
	if err := ps.Save(); err != nil {
		output.Warn("Failed to save push state: %v", err)
	}

	// Summarize.
	pushed, failed := 0, 0
	for _, r := range results {
		if r.Pushed {
			pushed++
		} else {
			failed++
		}
	}

	summary := fmt.Sprintf("Pushed %d skill(s)", pushed)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}

	return w.OK(results, output.WithSummary(summary))
}

// pushOneSkill stages, archives, and publishes a single skill with retry.
func pushOneSkill(ctx context.Context, s skillEntry, reg *registry.Client) (*registry.PublishResponse, error) {
	m, err := manifest.LoadFromDir(s.Dir)
	if err != nil {
		return nil, err
	}

	// Ensure push defaults.
	if m.Visibility == "" {
		m.Visibility = "private"
	}
	if m.Visibility == "private" && !m.Mutable {
		m.Mutable = true
	}

	data, err := manifest.Marshal(m)
	if err != nil {
		return nil, err
	}

	// Stage and create archive.
	archive, cleanup, err := stageAndArchive(s.Dir, data)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return publishWithRetry(ctx, reg, data, archive)
}

// stageAndArchive creates a staging directory from skill contents, creates a tar.gz archive,
// and returns the archive reader plus a cleanup function.
func stageAndArchive(dir string, manifestData []byte) (io.ReadSeeker, func(), error) {
	stg, err := staging.New("ctx-push-")
	if err != nil {
		return nil, nil, fmt.Errorf("create staging: %w", err)
	}

	if err := stg.CopyFrom(dir); err != nil {
		stg.Rollback()
		return nil, nil, fmt.Errorf("stage directory: %w", err)
	}

	// Remove build artifacts that should not be included in the archive.
	_ = os.Remove(filepath.Join(stg.Path, "package.tar.gz"))

	// Overwrite ctx.yaml with the (possibly modified) manifest.
	if err := stg.WriteFile(manifest.FileName, manifestData, 0o644); err != nil {
		stg.Rollback()
		return nil, nil, fmt.Errorf("stage manifest: %w", err)
	}

	archive, err := stg.TarGz()
	if err != nil {
		stg.Rollback()
		return nil, nil, fmt.Errorf("create archive: %w", err)
	}

	// archive is a *tempFileReader which embeds *os.File → supports Seek.
	seeker, ok := archive.(io.ReadSeeker)
	if !ok {
		_ = archive.Close()
		stg.Rollback()
		return nil, nil, fmt.Errorf("archive does not support seeking")
	}

	cleanup := func() {
		_ = archive.Close()
		stg.Rollback()
	}
	return seeker, cleanup, nil
}

// publishWithRetry wraps reg.Publish with exponential backoff for retryable errors.
func publishWithRetry(ctx context.Context, reg *registry.Client, data []byte, archive io.ReadSeeker, cfg ...retryConfig) (*registry.PublishResponse, error) {
	rc := defaultRetryConfig
	if len(cfg) > 0 {
		rc = cfg[0]
	}

	var lastErr error
	for attempt := 0; attempt <= rc.MaxRetries; attempt++ {
		if attempt > 0 {
			// Reset archive to beginning for retry.
			if _, err := archive.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("reset archive for retry: %w", err)
			}

			backoff := calcBackoff(attempt, rc)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := reg.Publish(ctx, data, archive)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if !isRetryable(err) {
			return nil, err
		}
		output.Warn("Publish failed (attempt %d/%d): %v", attempt+1, rc.MaxRetries+1, err)
	}
	return nil, fmt.Errorf("publish failed after %d attempts: %w", rc.MaxRetries+1, lastErr)
}

// calcBackoff returns the backoff duration for a given retry attempt (1-indexed).
func calcBackoff(attempt int, rc retryConfig) time.Duration {
	base := float64(rc.InitialBackoff) * math.Pow(rc.BackoffFactor, float64(attempt-1))
	if base > float64(rc.MaxBackoff) {
		base = float64(rc.MaxBackoff)
	}
	// Add 0-25% jitter.
	jitter := base * 0.25 * rand.Float64()
	return time.Duration(base + jitter)
}

// isRetryable returns true if the error indicates a transient failure.
func isRetryable(err error) bool {
	// Context cancellation is not retryable.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check output.CLIError (used by some error paths).
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Retryable
	}

	// Check registry.APIError (returned by registry client).
	var apiErr *registry.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}

	// Transport-layer errors (DNS, TCP, TLS, timeout) from http.Client.Do
	// surface as wrapped errors via "publish request: %w". These are transient
	// and should be retried.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Wrapped transport errors that don't implement net.Error (e.g. DNS failures
	// on some platforms) but still originate from the HTTP round-trip.
	if strings.Contains(err.Error(), "publish request:") {
		return true
	}

	return false
}

// pushDirectory handles pushing a single directory (explicit path or resolved name).
func pushDirectory(cmd *cobra.Command, dir string, w *output.Writer, cfg *config.Config, ps *pushstate.PushState, token string) error {
	// Check for ctx.yaml or SKILL.md.
	needsWrite := false
	yamlPath := filepath.Join(dir, manifest.FileName)
	var m *manifest.Manifest
	if _, statErr := os.Stat(yamlPath); statErr == nil {
		var loadErr error
		m, loadErr = manifest.LoadFromDir(dir)
		if loadErr != nil {
			return fmt.Errorf("ctx.yaml exists but cannot be parsed: %w\nFix the syntax errors before pushing", loadErr)
		}
	} else {
		// No ctx.yaml — check for SKILL.md for auto-init.
		skillPath := filepath.Join(dir, "SKILL.md")
		if _, skillStatErr := os.Stat(skillPath); skillStatErr != nil {
			return output.ErrUsageHint(
				"no ctx.yaml or SKILL.md found",
				"Run 'ctx init' to create a manifest, or create a SKILL.md",
			)
		}
		output.Info("Found SKILL.md, auto-creating ctx.yaml...")
		scope := cfg.Username
		if scope == "" {
			scope = "your-scope"
		}
		dirName := filepath.Base(dir)
		if abs, absErr := filepath.Abs(dir); absErr == nil {
			dirName = filepath.Base(abs)
		}
		m = manifest.Scaffold(manifest.TypeSkill, scope, dirName)
		m.Visibility = "private"
		m.Mutable = true
		needsWrite = true
	}

	// Apply version bump.
	if flagBump != "" {
		bumped, bumpErr := manifest.BumpVersion(m.Version, flagBump)
		if bumpErr != nil {
			return bumpErr
		}
		m.Version = bumped
		needsWrite = true
	}

	// Set push defaults.
	if m.Visibility == "" {
		m.Visibility = "private"
		needsWrite = true
	}
	if m.Visibility == "private" && !m.Mutable {
		m.Mutable = true
		needsWrite = true
	}

	// Auto-fill scope from logged-in user.
	if scope := m.Scope(); scope == "your-scope" || scope == "" {
		if cfg.Username != "" {
			_, name := manifest.ParseFullName(m.Name)
			m.Name = manifest.FormatFullName(cfg.Username, name)
			needsWrite = true
		}
	}

	// Validate before writing to disk to avoid leaving a modified ctx.yaml on error.
	errs := manifest.Validate(m)
	if len(errs) > 0 {
		return output.ErrUsageHint("validation failed: "+errs[0], "Fix errors and try again")
	}

	// Marshal manifest.
	data, err := manifest.Marshal(m)
	if err != nil {
		return err
	}
	if needsWrite {
		if writeErr := os.WriteFile(filepath.Join(dir, manifest.FileName), data, 0o644); writeErr != nil {
			return fmt.Errorf("write %s: %w", manifest.FileName, writeErr)
		}
	}

	// Dry-run: show what would happen.
	if flagPushDryRun {
		output.Header("Dry Run")
		output.Table([][]string{
			{"Name:", m.Name},
			{"Version:", m.Version},
			{"Directory:", dir},
		})
		return w.OK(map[string]string{
			"full_name": m.Name,
			"version":   m.Version,
			"dir":       dir,
		}, output.WithSummary(fmt.Sprintf("Would push %s@%s", m.Name, m.Version)))
	}

	// Stage, archive, and publish.
	reg := registry.New(cfg.RegistryURL(), token)
	output.Info("Pushing %s@%s...", m.Name, m.Version)

	archive, cleanup, err := stageAndArchive(dir, data)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := publishWithRetry(cmd.Context(), reg, data, archive)
	if err != nil {
		return err
	}

	// Record push state.
	hash, _ := pushstate.HashDir(dir)
	ps.RecordPush(m.Name, hash, result.Version, dir)
	if saveErr := ps.Save(); saveErr != nil {
		output.Warn("Failed to save push state: %v", saveErr)
	}

	return w.OK(result,
		output.WithSummary(fmt.Sprintf("Pushed %s@%s (private)", result.FullName, result.Version)),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "install", Command: "ctx install " + result.FullName, Description: "Install on another device"},
			output.Breadcrumb{Action: "sync", Command: "ctx sync push", Description: "Sync all packages to profile"},
		),
	)
}

package mcpclient

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TestResult summarises an MCP server health check.
type TestResult struct {
	Status     string          // "pass", "fail", "warn"
	Steps      []StepResult    // ordered test steps
	ServerInfo *ServerInfo     // from initialize, nil on failure
	Tools      []ToolInfo      // from tools/list, nil on failure
	Duration   time.Duration   // total test duration
	Stderr     string          // captured stderr (stdio only)
}

// StepResult describes the outcome of one test step.
type StepResult struct {
	Name    string        // "connect", "initialize", "tools/list", "validate"
	Status  string        // "pass", "fail", "skip"
	Detail  string        // human-readable detail
	Elapsed time.Duration // time spent on this step
}

// RunTest performs a full health check against an MCP server:
//  1. Connect — spawn the process or open HTTP connection
//  2. Initialize — MCP handshake
//  3. ListTools — retrieve available tools
//  4. Validate — compare returned tools against declaredTools (if non-empty)
//
// The returned TestResult always has len(Steps) >= 1. On first failure the
// remaining steps are recorded as "skip".
func RunTest(ctx context.Context, opts ConnectOptions, declaredTools []string) (*TestResult, error) {
	start := time.Now()
	result := &TestResult{}

	// Apply overall timeout
	timeout := opts.timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Step 1: Connect
	stepStart := time.Now()
	client, err := Connect(ctx, opts)
	connectStep := StepResult{Name: "connect", Elapsed: time.Since(stepStart)}
	if err != nil {
		connectStep.Status = "fail"
		connectStep.Detail = err.Error()
		result.Steps = append(result.Steps, connectStep)
		result.Status = "fail"
		result.Duration = time.Since(start)
		return result, nil
	}
	defer func() { _ = client.Close() }()
	connectStep.Status = "pass"
	connectStep.Detail = formatConnectDetail(opts)
	result.Steps = append(result.Steps, connectStep)

	// Step 2: Initialize
	stepStart = time.Now()
	initResult, err := client.Initialize(ctx)
	initStep := StepResult{Name: "initialize", Elapsed: time.Since(stepStart)}
	if err != nil {
		initStep.Status = "fail"
		initStep.Detail = err.Error()
		result.Steps = append(result.Steps, initStep)
		result.Steps = append(result.Steps, StepResult{Name: "tools/list", Status: "skip"})
		if len(declaredTools) > 0 {
			result.Steps = append(result.Steps, StepResult{Name: "validate", Status: "skip"})
		}
		result.Status = "fail"
		result.Stderr = client.Stderr()
		result.Duration = time.Since(start)
		return result, nil
	}
	initStep.Status = "pass"
	initStep.Detail = fmt.Sprintf("%s v%s (protocol %s)",
		initResult.ServerInfo.Name, initResult.ServerInfo.Version, initResult.ProtocolVersion)
	result.Steps = append(result.Steps, initStep)
	result.ServerInfo = &initResult.ServerInfo

	// Step 3: ListTools
	stepStart = time.Now()
	tools, err := client.ListTools(ctx)
	toolsStep := StepResult{Name: "tools/list", Elapsed: time.Since(stepStart)}
	if err != nil {
		toolsStep.Status = "fail"
		toolsStep.Detail = err.Error()
		result.Steps = append(result.Steps, toolsStep)
		if len(declaredTools) > 0 {
			result.Steps = append(result.Steps, StepResult{Name: "validate", Status: "skip"})
		}
		result.Status = "fail"
		result.Stderr = client.Stderr()
		result.Duration = time.Since(start)
		return result, nil
	}
	toolsStep.Status = "pass"
	toolsStep.Detail = fmt.Sprintf("%d tools available", len(tools))
	result.Steps = append(result.Steps, toolsStep)
	result.Tools = tools

	// Step 4: Validate (optional — only if declaredTools is non-empty)
	if len(declaredTools) > 0 {
		validateStep := validateTools(tools, declaredTools)
		result.Steps = append(result.Steps, validateStep)
		if validateStep.Status == "fail" {
			result.Status = "warn" // server works but manifest doesn't match
		}
	}

	result.Stderr = client.Stderr()
	result.Duration = time.Since(start)
	if result.Status == "" {
		result.Status = "pass"
	}
	return result, nil
}

// validateTools compares the actual tools returned by the server with the
// tools declared in the manifest.
func validateTools(actual []ToolInfo, declared []string) StepResult {
	actualSet := make(map[string]bool, len(actual))
	for _, t := range actual {
		actualSet[t.Name] = true
	}

	var missing, extra []string
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
		if !actualSet[name] {
			missing = append(missing, name)
		}
	}
	for _, t := range actual {
		if !declaredSet[t.Name] {
			extra = append(extra, t.Name)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)

	step := StepResult{Name: "validate"}
	if len(missing) == 0 && len(extra) == 0 {
		step.Status = "pass"
		step.Detail = fmt.Sprintf("all %d declared tools match", len(declared))
		return step
	}

	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing: %v", missing))
	}
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("undeclared: %v", extra))
	}
	step.Status = "fail"
	step.Detail = joinParts(parts)
	return step
}

func joinParts(parts []string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(p)
	}
	return b.String()
}

func formatConnectDetail(opts ConnectOptions) string {
	if opts.Transport == "http" || opts.Transport == "sse" || opts.Transport == "streamable-http" {
		return fmt.Sprintf("HTTP %s", opts.URL)
	}
	if len(opts.Args) > 0 {
		return fmt.Sprintf("%s %s", opts.Command, joinParts(opts.Args))
	}
	return opts.Command
}

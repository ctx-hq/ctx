package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ctx-hq/ctx/internal/auth"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/profile"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage authentication profiles",
	Long:  `Manage named authentication profiles for multi-account support.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default action: list profiles
		return profileListCmd.RunE(cmd, args)
	},
}

// ProfileListEntry is the JSON-serializable profile list entry.
type ProfileListEntry struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Registry string `json:"registry"`
	Active   bool   `json:"active"`
	Source   string `json:"source,omitempty"`
}

var profileListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all profiles",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		store, err := profile.Load()
		if err != nil {
			return err
		}

		if len(store.Profiles) == 0 {
			return w.OK([]ProfileListEntry{},
				output.WithSummary("No profiles configured"),
				output.WithBreadcrumbs(
					output.Breadcrumb{Action: "login", Command: "ctx login", Description: "Authenticate with getctx.org"},
				),
			)
		}

		// Resolve current profile for marking
		currentName := ""
		currentSource := ""
		if res, err := profile.Resolve(flagProfile); err == nil {
			currentName = res.Name
			currentSource = res.Source
		}

		var entries []ProfileListEntry
		for name, p := range store.Profiles {
			e := ProfileListEntry{
				Name:     name,
				Username: p.Username,
				Registry: p.RegistryURL(),
				Active:   name == currentName,
			}
			if name == currentName {
				e.Source = currentSource
			}
			entries = append(entries, e)
		}

		return w.OK(entries,
			output.WithSummary(fmt.Sprintf("%d profiles", len(entries))),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "use", Command: "ctx profile use <name>", Description: "Switch active profile"},
				output.Breadcrumb{Action: "add", Command: "ctx login --profile <name>", Description: "Add a new profile"},
			),
		)
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the global active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		name := args[0]

		store, err := profile.Load()
		if err != nil {
			return err
		}

		p, ok := store.Profiles[name]
		if !ok {
			return output.ErrUsageHint(
				fmt.Sprintf("profile %q not found", name),
				"Run 'ctx profile list' to see available profiles",
			)
		}

		store.Active = name
		if err := store.Save(); err != nil {
			return fmt.Errorf("save profiles: %w", err)
		}

		info := map[string]any{
			"name":     name,
			"username": p.Username,
			"registry": p.RegistryURL(),
		}

		summary := fmt.Sprintf("Switched to profile %q", name)
		if p.Username != "" {
			summary = fmt.Sprintf("Switched to profile %q (%s)", name, p.Username)
		}

		return w.OK(info,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "whoami", Command: "ctx whoami", Description: "Show current identity"},
			),
		)
	},
}

var profileRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a profile and its credentials",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		name := args[0]

		store, err := profile.Load()
		if err != nil {
			return err
		}

		p, ok := store.Profiles[name]
		if !ok {
			return output.ErrUsageHint(
				fmt.Sprintf("profile %q not found", name),
				"Run 'ctx profile list' to see available profiles",
			)
		}

		username := p.Username

		// Clear keychain token
		if err := auth.ClearProfileToken(name); err != nil {
			return err
		}

		// Remove from store
		delete(store.Profiles, name)
		if store.Active == name {
			store.Active = ""
		}
		if err := store.Save(); err != nil {
			return fmt.Errorf("save profiles: %w", err)
		}

		summary := fmt.Sprintf("Removed profile %q", name)
		if username != "" {
			summary = fmt.Sprintf("Removed profile %q (%s)", name, username)
		}

		return w.OK(map[string]any{"removed": name, "username": username},
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx profile list", Description: "List remaining profiles"},
			),
		)
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		var name string
		var p *profile.Profile
		var source string

		if len(args) == 1 {
			name = args[0]
			store, err := profile.Load()
			if err != nil {
				return err
			}
			var ok bool
			p, ok = store.Profiles[name]
			if !ok {
				return output.ErrUsageHint(
					fmt.Sprintf("profile %q not found", name),
					"Run 'ctx profile list' to see available profiles",
				)
			}
			source = "explicit"
		} else {
			// Show current resolved profile
			res, err := profile.Resolve(flagProfile)
			if err != nil {
				return output.ErrAuth("no active profile")
			}
			name = res.Name
			p = res.Profile
			source = res.Source
		}

		// Check if token exists
		hasToken := false
		if token, _ := auth.GetProfileToken(name); token != "" {
			hasToken = true
		}

		info := map[string]any{
			"name":      name,
			"username":  p.Username,
			"registry":  p.RegistryURL(),
			"source":    source,
			"has_token": hasToken,
		}

		return w.OK(info,
			output.WithSummary(fmt.Sprintf("Profile: %s (%s)", name, p.Username)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "use", Command: fmt.Sprintf("ctx profile use %s", name), Description: "Switch to this profile"},
			),
		)
	},
}

var profileLinkCmd = &cobra.Command{
	Use:   "link <name>",
	Short: "Bind current directory to a profile",
	Long:  `Creates a .ctx-profile file in the current directory, binding it to the named profile.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		name := args[0]

		// Verify profile exists
		store, err := profile.Load()
		if err != nil {
			return err
		}
		if _, ok := store.Profiles[name]; !ok {
			return output.ErrUsageHint(
				fmt.Sprintf("profile %q not found", name),
				"Run 'ctx profile list' to see available profiles",
			)
		}

		// Write .ctx-profile
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		path := filepath.Join(cwd, ".ctx-profile")
		if err := os.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
			return fmt.Errorf("write .ctx-profile: %w", err)
		}

		return w.OK(map[string]any{"profile": name, "path": path},
			output.WithSummary(fmt.Sprintf("Linked current directory to profile %q", name)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "unlink", Command: "ctx profile unlink", Description: "Remove project binding"},
			),
		)
	},
}

var profileUnlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Remove project profile binding",
	Long:  `Removes the .ctx-profile file from the current directory.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		path := filepath.Join(cwd, ".ctx-profile")

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return w.OK(map[string]any{"status": "not_linked"},
				output.WithSummary("No .ctx-profile in current directory"),
			)
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove .ctx-profile: %w", err)
		}

		return w.OK(map[string]any{"status": "unlinked", "path": path},
			output.WithSummary("Removed .ctx-profile from current directory"),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "link", Command: "ctx profile link <name>", Description: "Bind to a profile"},
			),
		)
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileRemoveCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileLinkCmd)
	profileCmd.AddCommand(profileUnlinkCmd)
}

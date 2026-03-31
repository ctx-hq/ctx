package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

// configurable keys and their valid values
var configKeys = map[string]struct {
	description string
	validate    func(string) error
}{
	"update_check": {
		description: "Enable/disable automatic update checks (true/false)",
		validate:    validateBool,
	},
	"network_mode": {
		description: "Network mode (online/offline)",
		validate: func(v string) error {
			if v != "online" && v != "offline" {
				return fmt.Errorf("must be 'online' or 'offline'")
			}
			return nil
		},
	},
	"registry": {
		description: "Registry URL",
		validate: func(v string) error {
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				return fmt.Errorf("must be a valid URL starting with http:// or https://")
			}
			return nil
		},
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ctx configuration",
	Long: `View and modify ctx configuration settings.

  ctx config list                 List all settings
  ctx config get <key>            Get a setting value
  ctx config set <key> <value>    Set a setting value`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		cfg, err := config.Load()
		if err != nil {
			return w.Err(output.ErrUsage(fmt.Sprintf("failed to load config: %v", err)))
		}

		settings := map[string]any{
			"registry":     cfg.RegistryURL(),
			"update_check": cfg.IsUpdateCheckEnabled(),
			"network_mode": cfg.NetworkMode,
			"username":     cfg.Username,
		}
		if settings["network_mode"] == "" {
			settings["network_mode"] = "online"
		}

		return w.OK(settings,
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "set", Command: "ctx config set <key> <value>", Description: "Change a setting"},
				output.Breadcrumb{Action: "doctor", Command: "ctx doctor", Description: "Check environment health"},
			),
		)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		key := args[0]

		cfg, err := config.Load()
		if err != nil {
			return w.Err(output.ErrUsage(fmt.Sprintf("failed to load config: %v", err)))
		}

		var value any
		switch key {
		case "registry":
			value = cfg.RegistryURL()
		case "update_check":
			value = cfg.IsUpdateCheckEnabled()
		case "network_mode":
			v := cfg.NetworkMode
			if v == "" {
				v = "online"
			}
			value = v
		case "username":
			value = cfg.Username
		default:
			return w.Err(output.ErrUsage(fmt.Sprintf("unknown config key: %s", key)))
		}

		return w.OK(map[string]any{key: value},
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx config list", Description: "View all settings"},
			),
		)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)
		key, value := args[0], args[1]

		info, ok := configKeys[key]
		if !ok {
			valid := make([]string, 0, len(configKeys))
			for k := range configKeys {
				valid = append(valid, k)
			}
			slices.Sort(valid)
			return w.Err(output.ErrUsage(fmt.Sprintf("unknown config key: %s (available: %s)", key, strings.Join(valid, ", "))))
		}

		if err := info.validate(value); err != nil {
			return w.Err(output.ErrUsage(fmt.Sprintf("invalid value for %s: %v", key, err)))
		}

		cfg, err := config.Load()
		if err != nil {
			return w.Err(output.ErrUsage(fmt.Sprintf("failed to load config: %v", err)))
		}

		switch key {
		case "update_check":
			b, _ := strconv.ParseBool(value)
			cfg.UpdateCheck = &b
		case "network_mode":
			cfg.NetworkMode = value
		case "registry":
			cfg.Registry = value
		}

		if err := cfg.Save(); err != nil {
			return w.Err(output.ErrUsage(fmt.Sprintf("failed to save config: %v", err)))
		}

		return w.OK(map[string]any{"set": key, "value": value},
			output.WithSummary(fmt.Sprintf("Set %s = %s", key, value)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "list", Command: "ctx config list", Description: "View all settings"},
			),
		)
	},
}

func validateBool(v string) error {
	if _, err := strconv.ParseBool(v); err != nil {
		return fmt.Errorf("must be 'true' or 'false'")
	}
	return nil
}

func init() {
	configCmd.AddCommand(configListCmd, configGetCmd, configSetCmd)
	rootCmd.AddCommand(configCmd)
}

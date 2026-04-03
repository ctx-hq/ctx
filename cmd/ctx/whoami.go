package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/profile"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

// WhoamiInfo is the response data for the whoami command.
type WhoamiInfo struct {
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Profile   string `json:"profile"`
	Source    string `json:"source"` // "api", "cached", or resolve source
	Registry  string `json:"registry"`
}

var flagWhoamiAll bool

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current authenticated user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		// --all: show all profiles
		if flagWhoamiAll {
			return whoamiAll(w)
		}

		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		res, err := profile.Resolve(flagProfile)
		if err != nil {
			if errors.Is(err, profile.ErrNoProfile) {
				return output.ErrAuth("not logged in")
			}
			return err
		}

		registryURL := res.Profile.RegistryURL()
		offline := flagOffline
		if !offline {
			if cfg, err := config.Load(); err == nil {
				offline = cfg.IsOffline()
			}
		}

		// Offline mode: use cached info only
		if offline {
			return whoamiCached(w, res)
		}

		// Online: try API with timeout
		apiCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		client := registry.New(registryURL, token)
		me, err := client.GetMe(apiCtx)
		if err != nil {
			var apiErr *registry.APIError
			if errors.As(err, &apiErr) {
				switch {
				case apiErr.StatusCode == 401:
					return output.ErrAuth("token expired or revoked")
				case apiErr.StatusCode >= 500:
					if res.Profile.Username != "" {
						if w.IsStyled() {
							output.Warn("registry returned server error, showing cached info")
						}
						return whoamiCached(w, res)
					}
					return output.FromHTTPStatus(apiErr.StatusCode, apiErr.Msg)
				default:
					return output.FromHTTPStatus(apiErr.StatusCode, apiErr.Msg)
				}
			}

			// JSON decode error
			var jsonErr *json.SyntaxError
			var jsonTypeErr *json.UnmarshalTypeError
			if errors.As(err, &jsonErr) || errors.As(err, &jsonTypeErr) {
				return output.ErrAPI(502, "unexpected response from registry")
			}

			// True network error: fall back to cached
			if res.Profile.Username != "" {
				if w.IsStyled() {
					output.Warn("could not reach registry, showing cached info")
				}
				return whoamiCached(w, res)
			}
			return output.ErrNetwork(err)
		}

		// Side effect: update cached username if it changed
		if me.Username != res.Profile.Username {
			store, loadErr := profile.Load()
			if loadErr == nil {
				if p, ok := store.Profiles[res.Name]; ok {
					p.Username = me.Username
					_ = store.Save()
				}
			}
		}

		info := &WhoamiInfo{
			Username:  me.Username,
			Email:     me.Email,
			AvatarURL: me.AvatarURL,
			Profile:   res.Name,
			Source:    "api",
			Registry:  registryURL,
		}

		summary := fmt.Sprintf("logged in as %s (profile: %s)", me.Username, res.Name)
		if res.Source != "global" && res.Source != "default" {
			summary += fmt.Sprintf(" [from %s]", res.Source)
		}

		return w.OK(info,
			output.WithSummary(summary),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "switch", Command: "ctx profile use <name>", Description: "Switch active profile"},
				output.Breadcrumb{Action: "profiles", Command: "ctx profile list", Description: "List all profiles"},
			),
		)
	},
}

func init() {
	whoamiCmd.Flags().BoolVar(&flagWhoamiAll, "all", false, "Show all profiles")
}

func whoamiCached(w *output.Writer, res *profile.ResolveResult) error {
	if res.Profile.Username == "" {
		return output.ErrAuth("not logged in (no cached username)")
	}
	info := &WhoamiInfo{
		Username: res.Profile.Username,
		Profile:  res.Name,
		Source:   res.Source,
		Registry: res.Profile.RegistryURL(),
	}
	return w.OK(info,
		output.WithSummary(fmt.Sprintf("logged in as %s (profile: %s, cached)", res.Profile.Username, res.Name)),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "switch", Command: "ctx profile use <name>", Description: "Switch active profile"},
		),
	)
}

func whoamiAll(w *output.Writer) error {
	store, err := profile.Load()
	if err != nil {
		return err
	}

	if len(store.Profiles) == 0 {
		return output.ErrAuth("no profiles configured")
	}

	// Resolve current active for marking
	currentName := ""
	currentSource := ""
	if res, err := profile.Resolve(flagProfile); err == nil {
		currentName = res.Name
		currentSource = res.Source
	}

	type profileEntry struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Registry string `json:"registry"`
		Active   bool   `json:"active"`
		Source   string `json:"source,omitempty"`
	}

	var entries []profileEntry
	for name, p := range store.Profiles {
		e := profileEntry{
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
			output.Breadcrumb{Action: "switch", Command: "ctx profile use <name>", Description: "Switch active profile"},
			output.Breadcrumb{Action: "login", Command: "ctx login --profile <name>", Description: "Add a new profile"},
		),
	)
}


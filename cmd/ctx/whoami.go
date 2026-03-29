package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ctx-hq/ctx/internal/config"
	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

// WhoamiInfo is the response data for the whoami command.
type WhoamiInfo struct {
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Registry  string `json:"registry"`
	Source    string `json:"source"` // "api" or "cached"
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current authenticated user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriter(cmd)

		token := getToken()
		if token == "" {
			return output.ErrAuth("not logged in")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		registryURL := cfg.RegistryURL()
		offline := flagOffline || cfg.IsOffline()

		// Offline mode: use cached info only
		if offline {
			return whoamiCached(w, cfg, registryURL)
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
					// Server error: fall back to cached username
					if cfg.Username != "" {
						if w.IsStyled() {
							output.Warn("registry returned server error, showing cached info")
						}
						return whoamiCached(w, cfg, registryURL)
					}
					return output.FromHTTPStatus(apiErr.StatusCode, apiErr.Msg)
				default:
					return output.FromHTTPStatus(apiErr.StatusCode, apiErr.Msg)
				}
			}

			// JSON decode error (e.g. 200 OK with malformed body) — not a network issue.
			var jsonErr *json.SyntaxError
			var jsonTypeErr *json.UnmarshalTypeError
			if errors.As(err, &jsonErr) || errors.As(err, &jsonTypeErr) {
				return output.ErrAPI(502, "unexpected response from registry")
			}

			// True network/connection error: fall back to cached username
			if cfg.Username != "" {
				if w.IsStyled() {
					output.Warn("could not reach registry, showing cached info")
				}
				return whoamiCached(w, cfg, registryURL)
			}
			return output.ErrNetwork(err)
		}

		// Side effect: update cached username if it changed
		if me.Username != cfg.Username {
			cfg.Username = me.Username
			if err := cfg.Save(); err != nil && w.IsStyled() {
				output.Warn("failed to update cached username: " + err.Error())
			}
		}

		info := &WhoamiInfo{
			Username:  me.Username,
			Email:     me.Email,
			AvatarURL: me.AvatarURL,
			Registry:  registryURL,
			Source:    "api",
		}

		return w.OK(info,
			output.WithSummary(fmt.Sprintf("logged in as %s", me.Username)),
			output.WithBreadcrumbs(
				output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish a package"},
			),
		)
	},
}

func whoamiCached(w *output.Writer, cfg *config.Config, registryURL string) error {
	if cfg.Username == "" {
		return output.ErrAuth("not logged in (no cached username)")
	}
	info := &WhoamiInfo{
		Username: cfg.Username,
		Registry: registryURL,
		Source:   "cached",
	}
	return w.OK(info,
		output.WithSummary(fmt.Sprintf("logged in as %s (cached)", cfg.Username)),
		output.WithBreadcrumbs(
			output.Breadcrumb{Action: "publish", Command: "ctx publish", Description: "Publish a package"},
		),
	)
}

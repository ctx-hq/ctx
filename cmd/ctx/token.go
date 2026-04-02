package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/ctx-hq/ctx/internal/registry"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage API tokens for CI/CD and automation",
	Long: `Create and manage API tokens with scoped permissions.

Tokens enable CI/CD publishing (e.g. GitHub Actions) and machine-to-machine
access to private packages.

Examples:
  ctx token create --name "github-ci" --scope publish --package "@myorg/*" --expires 90
  ctx token list
  ctx token revoke <token-id>`,
}

var (
	tokenName    string
	tokenScope   []string
	tokenPackage []string
	tokenExpires int
	tokenType    string
)

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, client, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if tokenName == "" {
			return fmt.Errorf("--name is required")
		}

		req := registry.CreateTokenRequest{
			Name: tokenName,
		}

		if tokenExpires > 0 {
			req.ExpiresInDays = tokenExpires
		}

		if len(tokenScope) > 0 {
			req.EndpointScopes = tokenScope
		}

		if len(tokenPackage) > 0 {
			req.PackageScopes = tokenPackage
		}

		if tokenType != "" {
			req.TokenType = tokenType
		}

		result, err := client.CreateToken(cmd.Context(), req)
		if err != nil {
			return err
		}

		// Show the token prominently on stderr — it's only returned once.
		// Avoid printing to stdout so JSON mode output stays clean.
		fmt.Fprintf(cmd.ErrOrStderr(), "\n  %s\n\nSave this token now — it will not be shown again.\n", result.Token)

		return w.OK(map[string]string{
			"id":   result.ID,
			"name": result.Name,
		}, output.WithSummary("Token created: "+result.Name))
	},
}

var tokenListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List your API tokens",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		w, client, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		tokens, err := client.ListTokens(cmd.Context())
		if err != nil {
			return err
		}

		return w.OK(tokens, output.WithSummary(fmt.Sprintf("%d token(s)", len(tokens))))
	},
}

var tokenRevokeCmd = &cobra.Command{
	Use:   "revoke <token-id>",
	Short: "Revoke an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, client, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := client.RevokeToken(cmd.Context(), args[0]); err != nil {
			return err
		}

		return w.OK(map[string]string{"id": args[0], "status": "revoked"},
			output.WithSummary("Token revoked"))
	},
}

func init() {
	tokenCreateCmd.Flags().StringVar(&tokenName, "name", "", "Token name (required)")
	tokenCreateCmd.Flags().StringSliceVar(&tokenScope, "scope", nil, "Endpoint scopes: publish, yank, read-private, manage-access, manage-org")
	tokenCreateCmd.Flags().StringSliceVar(&tokenPackage, "package", nil, "Package scope patterns: @scope/*, @scope/name")
	tokenCreateCmd.Flags().IntVar(&tokenExpires, "expires", 0, "Expiry in days (0 = no expiry)")
	tokenCreateCmd.Flags().StringVar(&tokenType, "type", "", "Token type: personal (default) or deploy")

	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenRevokeCmd)
	rootCmd.AddCommand(tokenCmd)
}

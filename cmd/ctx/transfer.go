package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var transferMessage string

var transferCmd = &cobra.Command{
	Use:   "transfer <package> <target-scope>",
	Short: "Transfer a package to another scope",
	Long: `Transfer ownership of a package to another user or organization.

The target scope owner must accept the transfer request.

Examples:
  ctx transfer @alice/tool @acme
  ctx transfer @alice/tool @bob --message "Taking over maintenance"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		result, err := reg.InitiateTransfer(cmd.Context(), args[0], args[1], transferMessage)
		if err != nil {
			return err
		}
		return w.OK(result,
			output.WithSummary(fmt.Sprintf("Transfer request created: %s → %s", result.Package, result.To)),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "view",
				Command:     "ctx transfers",
				Description: "View pending transfers",
			}),
		)
	},
}

var transfersCmd = &cobra.Command{
	Use:     "transfers",
	Aliases: []string{"xfer"},
	Short:   "List incoming transfer requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		transfers, err := reg.ListMyTransfers(cmd.Context())
		if err != nil {
			return err
		}
		return w.OK(transfers,
			output.WithSummary(fmt.Sprintf("%d pending transfer(s)", len(transfers))),
		)
	},
}

var transferAcceptCmd = &cobra.Command{
	Use:   "accept <transfer-id>",
	Short: "Accept a transfer request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.AcceptTransfer(cmd.Context(), args[0]); err != nil {
			return err
		}
		return w.OK(map[string]string{"accepted": args[0]},
			output.WithSummary("Transfer accepted"),
		)
	},
}

var transferDeclineCmd = &cobra.Command{
	Use:   "decline <transfer-id>",
	Short: "Decline a transfer request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.DeclineTransfer(cmd.Context(), args[0]); err != nil {
			return err
		}
		return w.OK(map[string]string{"declined": args[0]},
			output.WithSummary("Transfer declined"),
		)
	},
}

var transferCancelCmd = &cobra.Command{
	Use:   "cancel <package>",
	Short: "Cancel a pending transfer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.CancelTransfer(cmd.Context(), args[0]); err != nil {
			return err
		}
		return w.OK(map[string]string{"cancelled": args[0]},
			output.WithSummary("Transfer cancelled"),
		)
	},
}

func init() {
	transferCmd.Flags().StringVar(&transferMessage, "message", "", "Optional message for the transfer request")

	transfersCmd.AddCommand(transferAcceptCmd)
	transfersCmd.AddCommand(transferDeclineCmd)
	transfersCmd.AddCommand(transferCancelCmd)

	rootCmd.AddCommand(transferCmd)
	rootCmd.AddCommand(transfersCmd)
}

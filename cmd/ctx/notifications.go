package main

import (
	"fmt"

	"github.com/ctx-hq/ctx/internal/output"
	"github.com/spf13/cobra"
)

var notifAll bool

var notificationsCmd = &cobra.Command{
	Use:     "notifications",
	Aliases: []string{"notif"},
	Short:   "List notifications",
	Long: `View your notifications (invitations, transfers, alerts).

Examples:
  ctx notifications           # Show unread
  ctx notifications --all     # Show all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		notifications, err := reg.ListNotifications(cmd.Context(), !notifAll)
		if err != nil {
			return err
		}
		return w.OK(notifications,
			output.WithSummary(fmt.Sprintf("%d notification(s)", len(notifications))),
		)
	},
}

var notifReadCmd = &cobra.Command{
	Use:   "read <id>",
	Short: "Mark a notification as read",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.MarkNotificationRead(cmd.Context(), args[0]); err != nil {
			return err
		}
		return w.OK(map[string]string{"read": args[0]},
			output.WithSummary("Notification marked as read"),
		)
	},
}

var notifDismissCmd = &cobra.Command{
	Use:   "dismiss <id>",
	Short: "Dismiss a notification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, reg, err := authedRegistry(cmd)
		if err != nil {
			return err
		}

		if err := reg.DismissNotification(cmd.Context(), args[0]); err != nil {
			return err
		}
		return w.OK(map[string]string{"dismissed": args[0]},
			output.WithSummary("Notification dismissed"),
		)
	},
}

func init() {
	notificationsCmd.Flags().BoolVar(&notifAll, "all", false, "Show all notifications (not just unread)")

	notificationsCmd.AddCommand(notifReadCmd)
	notificationsCmd.AddCommand(notifDismissCmd)

	rootCmd.AddCommand(notificationsCmd)
}

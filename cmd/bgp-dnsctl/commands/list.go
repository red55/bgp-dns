package commands

import (
	"context"
	"time"

	"github.com/red55/bgp-dns/api"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/spf13/cobra"
)

func newListCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Domain list management commands",
		Long:  "Commands to manage the domain list file.",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	cmd.AddCommand(
		newListReloadCmd(_app, client),
	)

	return cmd
}

func newListReloadCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload the domain list file",
		Long:  "Trigger the daemon to reload the configured domain list file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			_, err := (*client).ReloadList(ctx, &api.ReloadListRequest{})
			return err
		},
	}
}

package commands

import (
	"path/filepath"

	"github.com/red55/bgp-dns/api"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	client api.BgpDnsServiceClient
)

func NewRootCmd(_app *app.Application) *cobra.Command {
	cobra.EnablePrefixMatching = true
	cleanup := func() {}
	cmd := &cobra.Command{
		Use:   filepath.Base(_app.Name()),
		Short: "BGP DNS Control CLI",
		Long:  "A command line interface for controlling BGP DNS service.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var conn, e = grpc.NewClient(_app.Flags.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if e != nil {
				return e
			}

			client = api.NewBgpDnsServiceClient(conn)
			cleanup = func() {
				if err := conn.Close(); err != nil {
					_app.L().Fatal().Msgf("Failed to close connection %e", err)
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			defer cleanup()
		},
	}
	cmd.PersistentFlags().StringVarP(&_app.Flags.Target, "target", "t",
		app.DefaultTarget(),
		app.TargetDescription)

	cmd.AddCommand(
		newCacheCmd(_app, &client),
	)

	return cmd
}

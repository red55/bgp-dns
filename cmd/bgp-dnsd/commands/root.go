package commands

import (
	"context"
	"path/filepath"

	"github.com/red55/bgp-dns/api"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/spf13/cobra"
)

var (
	ctx    context.Context
	client api.BgpDnsServiceClient
)

func NewRootCmd(_app *app.Application) *cobra.Command {
	cobra.EnablePrefixMatching = true
	// cleanup := func() {}
	cmd := &cobra.Command{
		Use:   filepath.Base(_app.Name()),
		Short: "BGP DNS Service",
		Long:  "BGP DNS service.",
	}

	cmd.PersistentFlags().StringVarP(&_app.Flags.Target, "target", "t",
		app.DefaultTarget(),
		app.TargetDescription)
	cmd.PersistentFlags().StringVarP(&_app.Flags.Config, "config", "c",
		"appsettings.yml",
		"Path to configuration file.")

	return cmd
}

package commands

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/red55/bgp-dns/api"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func newCacheCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache management commands",
		Long:  "Commands to manage the DNS cache.",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	cmd.AddCommand(
		newCacheListCmd(_app, client),
		newCacheClearCmd(_app, client),
	)

	return cmd
}
func emptyArr(arr []string) []string {
	if len(arr) == 0 {
		return []string{"None"}
	}
	return arr
}
func newCacheListCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List DNS cache entries",
		Long:  "List all entries in the DNS cache.",
		RunE: func(cmd *cobra.Command, args []string) (e error) {
			// Implementation for listing cache entries
			_, _ = os.Stdout.WriteString("Listing DNS cache entries...\n")
			var stream api.BgpDnsService_ListCacheEntriesClient
			if stream, e = (*client).ListCacheEntries(context.Background(), &emptypb.Empty{}); e != nil {
				return e
			}

			for {
				r, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				_app.L().Info().Msgf("qtype: %s, fqdn:%s fails:%d addr:%s ttl:%d exp:%s gen:%s",
					r.Type,
					r.Fqdn,
					r.Fails,
					strings.Join(emptyArr(r.Addr), ","),
					r.Ttl,
					time.UnixMilli(r.Expiration).Format(time.RFC3339),
					r.Generation)
			}
			return nil
		},
	}

	return cmd
}

func newCacheClearCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear DNS cache entries",
		Long:  "Clear all entries in the DNS cache and withdraw associated IPs from BGP peers.",
		RunE: func(cmd *cobra.Command, args []string) (e error) {
			resp, err := (*client).ClearCache(context.Background(), &api.ClearCacheRequest{})
			if err != nil {
				return err
			}
			_app.L().Info().Msgf("Cleared %d cache entries.", resp.ClearedCount)
			return nil
		},
	}

	return cmd
}

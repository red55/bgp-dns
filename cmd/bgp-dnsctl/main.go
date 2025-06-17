package main

import (
	"github.com/red55/bgp-dns/cmd/bgp-dnsctl/commands"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/red55/bgp-dns/internal/version"
	"github.com/rs/zerolog"
)

var _app *app.Application

func main() {
	_app = app.New("bgp-dnsctl", zerolog.InfoLevel)
	_app.StdOut("%s %s", _app.Name(), version.Version())

	if e := commands.NewRootCmd(_app).Execute(); e != nil {
		_app.StdErr(e, "error executing command")
	}
}

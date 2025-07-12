package app

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/red55/bgp-dns/internal/log"
	"github.com/rs/zerolog"
)

const svcName = "bgp-dnsd"
const defaultTarget = "unix:///run/" + svcName + "/" + svcName + ".sock"
const TargetDescription = "BGP DNS Service socket/URI. Unix sockets should be prefixed with 'unix://'. " +
	"Defaults to " + defaultTarget + ".Or unix:///run/user/<uid>/bgp-dnsd.sock if the default path does not exist."

type GlobalFlags struct {
	Target string
	Config string
}
type Application struct {
	log.Log
	Flags *GlobalFlags
}

func DefaultTarget() string {

	var target = strings.TrimPrefix(defaultTarget, "unix://")
	if _, err := os.Stat(filepath.Dir(target)); err != nil && os.IsNotExist(err) {
		var uid = "0"
		if u, _ := user.Current(); u != nil {
			uid = u.Uid
		}
		target = "unix://" + filepath.Join("/run", "user", uid, svcName+".sock")

		return target
	}

	return defaultTarget
}

func New(module string, defaultLogLevel zerolog.Level) *Application {
	log.Init(defaultLogLevel)
	return &Application{
		Log: log.NewLog(log.L(), module),
		Flags: &GlobalFlags{
			Target: DefaultTarget(),
		},
	}
}

func (app *Application) Name() string {
	return filepath.Base(os.Args[0])
}

func (a *Application) StdErr(e error, format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	a.L().Error().Err(e).Msg(s)
	if a.L().GetLevel() >= zerolog.ErrorLevel {
		_, _ = fmt.Fprintf(os.Stderr, "%s - %v\n", s, e)
	}

}

func (a *Application) StdOut(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	a.L().Info().Msg(s)
	if a.L().GetLevel() >= zerolog.InfoLevel {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", s)
	}
}

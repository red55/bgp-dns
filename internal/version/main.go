package version

import (
	"fmt"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func Version() string {
	msg := fmt.Sprintf("%s (%s) built on %s", version, commit, date)
	return msg
}

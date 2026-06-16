package version

import (
	"fmt"
)

const unknown = "unknown"

var (
	version = "dev"
	commit  = "none"
	date    = unknown
	builtBy = unknown
)

func Version() string {
	return fmt.Sprintf("%s (%s) built at %s by %s", version, commit, date, builtBy)
}

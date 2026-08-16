//go:build linux

package debugdetect

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// UnderDebugger returns true when TracerPid is not zero.
func UnderDebugger() (bool, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return false, err
	}
	defer f.Close()

	return parseTracerPID(f)
}

func parseTracerPID(r io.Reader) (bool, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "TracerPid:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return false, nil
			}
			pid, err := strconv.Atoi(fields[1])
			if err != nil {
				return false, err
			}
			return pid != 0, nil
		}
	}
	if err := sc.Err(); err != nil {
		return false, err
	}
	return false, nil
}


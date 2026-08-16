//go:build windows

package debugdetect

import "syscall"

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procIsDebuggerPresent = kernel32.NewProc("IsDebuggerPresent")
)

// UnderDebugger returns true when Windows reports an attached debugger.
func UnderDebugger() (bool, error) {
	r1, _, err := procIsDebuggerPresent.Call()
	if r1 == 0 {
		if err != syscall.Errno(0) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}


//go:build darwin

package debugdetect

// UnderDebugger is currently a safe fallback on macOS.
func UnderDebugger() (bool, error) {
	return false, nil
}


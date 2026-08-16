//go:build !linux && !windows && !darwin

package debugdetect

// UnderDebugger is a no-op fallback for unsupported platforms.
func UnderDebugger() (bool, error) {
	return false, nil
}


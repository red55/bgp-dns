//go:build linux

package debugdetect

import (
	"strings"
	"testing"
)

func TestParseTracerPID_NotDebugged(t *testing.T) {
	in := "Name:\ttest\nTracerPid:\t0\n"
	got, err := parseTracerPID(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Fatalf("expected false, got true")
	}
}

func TestParseTracerPID_Debugged(t *testing.T) {
	in := "Name:\ttest\nTracerPid:\t1234\n"
	got, err := parseTracerPID(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatalf("expected true, got false")
	}
}


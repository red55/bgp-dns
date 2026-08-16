package dns

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
)

func initTestLogger() {
	log.Init(zerolog.WarnLevel)
}

func newTestCache(t *testing.T) *cache {
	t.Helper()
	initTestLogger()
	l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	return newCache(100, time.Duration(60), newResolversWithLogger(&l), &l, config.TestConfig(), loop.NewLoop(1, &l))
}

func newResolversWithLogger(l *zerolog.Logger) *resolvers {
	r := &resolvers{
		Log: log.NewLog(l, "resolvers"),
	}
	r.setResolvers(nil)
	return r
}

// TestCache_Load_FileNotFound verifies that load returns an error for a nonexistent file.
func TestCache_Load_FileNotFound(t *testing.T) {
	c := newTestCache(t)
	err := c.load("/nonexistent/path/domains.lst")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestCache_Load_GenerationIncrease verifies that loading a file increases the generation.
func TestCache_Load_GenerationIncrease(t *testing.T) {
	c := newTestCache(t)

	initialGen := c.generation()

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "test.lst")
	err := os.WriteFile(listFile, []byte("example.com\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_ = c.load(listFile)

	if c.generation() <= initialGen {
		t.Errorf("expected generation to increase from %d, got %d", initialGen, c.generation())
	}
}

// TestCache_Load_MultipleLoads verifies that multiple loads each increase generation.
func TestCache_Load_MultipleLoads(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "test.lst")

	err := os.WriteFile(listFile, []byte("example.com\ntest.com\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_ = c.load(listFile)
	gen1 := c.generation()

	_ = c.load(listFile)

	if c.generation() <= gen1 {
		t.Errorf("expected generation to increase from %d, got %d", gen1, c.generation())
	}
}

// TestCache_Load_BlankLines verifies blank lines in the file don't cause errors.
func TestCache_Load_BlankLines(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "blanks.lst")
	content := "example.com\n\ntest.com\n\nanother.com\n"
	err := os.WriteFile(listFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	err = c.load(listFile)
	if err != nil {
		t.Errorf("unexpected error loading file with blank lines: %v", err)
	}
}

// TestCache_Load_FileWithComments verifies comment lines are skipped.
func TestCache_Load_FileWithComments(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "comments.lst")
	content := "# This is a comment\n; This is also a comment\n\n   # Indented comment\n"
	err := os.WriteFile(listFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	err = c.load(listFile)
	if err != nil {
		t.Errorf("unexpected error loading file with comments: %v", err)
	}
}

// TestCache_Load_EmptyFile verifies loading an empty file doesn't error.
func TestCache_Load_EmptyFile(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "empty.lst")
	err := os.WriteFile(listFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	err = c.load(listFile)
	if err != nil {
		t.Errorf("unexpected error loading empty file: %v", err)
	}
}

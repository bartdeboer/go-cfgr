//go:build linux

package safety_test

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/bartdeboer/go-cfgr/storage/jsondir"
)

func TestJSONDirectoryRejectsNamedPipesBeforeReading(t *testing.T) {
	root := t.TempDir()
	layers := filepath.Join(root, "config.d")
	if err := syscall.Mkdir(layers, 0o700); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(layers, "10-block.json")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := jsondir.New(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = directory.ReadContents(context.Background(), "config.d")
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("ReadContents() error = %v, want non-regular-file error", err)
	}
}

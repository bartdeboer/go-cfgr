package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStorageRoundTrip(t *testing.T) {
	storage, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := storage.WriteContents(ctx, "groups/a/settings.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	data, err := storage.ReadContents(ctx, "groups/a/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("ReadContents() = %s", data)
	}
}

func TestStorageRejectsEscapeAndAbsoluteKeys(t *testing.T) {
	storage, _ := New(t.TempDir())
	for _, key := range []string{"../outside.json", "/absolute.json", "a/../../outside.json", ""} {
		if err := storage.WriteContents(context.Background(), key, []byte(`{}`)); err == nil {
			t.Errorf("WriteContents(%q) succeeded", key)
		}
	}
}

func TestStorageRejectsSymlinkComponents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	storage, _ := New(root)
	if err := storage.WriteContents(context.Background(), "escape/file.json", []byte(`{}`)); err == nil {
		t.Fatal("write through symlink succeeded")
	}
}

func TestStorageHonorsCancelledContext(t *testing.T) {
	storage, _ := New(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := storage.WriteContents(ctx, "x.json", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteContents() error = %v", err)
	}
}

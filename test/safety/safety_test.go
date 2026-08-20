package safety_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bartdeboer/go-cfgr"
	"github.com/bartdeboer/go-cfgr/storage/jsondir"
)

type documentStorage struct {
	contents []byte
	writes   int
}

func (s *documentStorage) ReadContents(context.Context, string) ([]byte, error) {
	return append([]byte(nil), s.contents...), nil
}

func (s *documentStorage) WriteContents(_ context.Context, _ string, contents []byte) error {
	s.writes++
	s.contents = append([]byte(nil), contents...)
	return nil
}

func TestPatchCannotProbeContentsWithoutReadAccess(t *testing.T) {
	storage := &documentStorage{contents: []byte("token: secret\n")}
	cfg := cfgr.New(cfgr.WithDefaultStorage(storage))
	settings := cfgr.NewRoute(
		cfg,
		cfgr.WithContentsReadAccess(func(context.Context, cfgr.NoParams) (bool, error) {
			return false, nil
		}),
	)
	patch := `*** Begin Patch
@@
-token: secret
+token: replaced
*** End Patch`

	err := settings.PatchContents(context.Background(), patch)
	if !errors.Is(err, cfgr.ErrDenied) {
		t.Fatalf("PatchContents() error = %v, want ErrDenied", err)
	}
	if storage.writes != 0 || string(storage.contents) != "token: secret\n" {
		t.Fatalf("writes=%d contents=%q", storage.writes, storage.contents)
	}
}

func TestJSONDirectoryRejectsUnsafeLocationsAndLayers(t *testing.T) {
	root := t.TempDir()
	directory, err := jsondir.New(root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := directory.ReadContents(context.Background(), "../outside"); err == nil {
		t.Fatal("ReadContents() accepted an escaping location")
	}

	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, "linked")
	if err := os.Symlink(realDirectory, symlink); err == nil {
		if _, err := directory.ReadContents(context.Background(), "linked"); err == nil {
			t.Fatal("ReadContents() accepted a symlink directory")
		}
	}

	layers := filepath.Join(root, "config.d")
	if err := os.Mkdir(layers, 0o700); err != nil {
		t.Fatal(err)
	}
	badLayer := filepath.Join(layers, "10-bad.json")
	if err := os.WriteFile(badLayer, []byte(`{not-json}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = directory.ReadContents(context.Background(), "config.d")
	if err == nil || !strings.Contains(err.Error(), "10-bad.json") {
		t.Fatalf("invalid layer error = %v, want filename", err)
	}
	if err := os.WriteFile(badLayer, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = directory.ReadContents(context.Background(), "config.d")
	if err == nil || !strings.Contains(err.Error(), "root must be an object") {
		t.Fatalf("non-object layer error = %v", err)
	}
	if err := os.Remove(badLayer); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(layers, "10-link.json")); err == nil {
		_, err = directory.ReadContents(context.Background(), "config.d")
		if err == nil || !strings.Contains(err.Error(), "symlink layer") {
			t.Fatalf("symlink layer error = %v", err)
		}
	}
}

func TestJSONDirectoryTreatsMissingAndEmptyDirectoriesAsEmptyObjects(t *testing.T) {
	root := t.TempDir()
	directory, err := jsondir.New(root)
	if err != nil {
		t.Fatal(err)
	}

	missing, err := directory.ReadContents(context.Background(), "missing.d")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty.d"), 0o700); err != nil {
		t.Fatal(err)
	}
	empty, err := directory.ReadContents(context.Background(), "empty.d")
	if err != nil {
		t.Fatal(err)
	}
	if string(missing) != `{}` || string(empty) != `{}` {
		t.Fatalf("missing=%s empty=%s, want {}", missing, empty)
	}
}

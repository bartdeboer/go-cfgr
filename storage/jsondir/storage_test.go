package jsondir_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bartdeboer/go-cfgr"
	fsstore "github.com/bartdeboer/go-cfgr/storage/fs"
	jsonstore "github.com/bartdeboer/go-cfgr/storage/json"
	"github.com/bartdeboer/go-cfgr/storage/jsondir"
)

type fragmentRouteParams struct{ Name string }

func TestFragmentRouteWritesFilesAndMergedRouteReadsTheirOverlay(t *testing.T) {
	root := t.TempDir()
	files, err := fsstore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	mergedDirectory, err := jsondir.New(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(files)))
	cfg.RegisterAdapter("merged-directory", jsonstore.NewReader(mergedDirectory))
	fragments := cfgr.NewRouteAs[fragmentRouteParams](
		cfg,
		"config-fragment",
		cfgr.WithLocationBuilder(func(params fragmentRouteParams) (string, error) {
			if params.Name == "" || filepath.Base(params.Name) != params.Name || strings.Contains(params.Name, "..") {
				return "", errors.New("invalid fragment name")
			}
			return "config.d/" + params.Name + ".json", nil
		}),
	)
	config := cfgr.NewRoute(
		cfg,
		"config",
		cfgr.WithAdapter[cfgr.NoParams]("merged-directory"),
		cfgr.WithLocationBuilder(func(cfgr.NoParams) (string, error) {
			return "config.d", nil
		}),
		cfgr.WithContentsWriteDisabled[cfgr.NoParams](),
	)

	if err := fragments.WriteContents(context.Background(), fragmentRouteParams{Name: "20-local"}, []byte(`{"server":{"port":9090},"items":["local"],"enabled":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := fragments.WriteContents(context.Background(), fragmentRouteParams{Name: "10-defaults"}, []byte(`{"server":{"host":"127.0.0.1","port":8080},"items":["default"]}`)); err != nil {
		t.Fatal(err)
	}

	contents, err := config.ReadContents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"server":{"host":"127.0.0.1","port":9090},"items":["local"],"enabled":true}`
	if string(contents) != want {
		t.Fatalf("merged contents = %s, want %s", contents, want)
	}
	port, err := config.ReadInt(context.Background(), "/server/port")
	if err != nil {
		t.Fatal(err)
	}
	if port != 9090 {
		t.Fatalf("port = %d, want 9090", port)
	}
}

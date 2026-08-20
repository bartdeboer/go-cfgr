package cfgr_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/bartdeboer/go-cfgr"
	jsonstore "github.com/bartdeboer/go-cfgr/storage/json"
)

// memoryContents is irrelevant test infrastructure. Every test keeps route
// registration visible because that is part of the consumer API story.
type memoryContents struct {
	mu     sync.Mutex
	data   map[string][]byte
	reads  int
	writes int
}

type readOnlyContents struct {
	data map[string][]byte
}

func (s *readOnlyContents) ReadContents(_ context.Context, document string) ([]byte, error) {
	contents, ok := s.data[document]
	if !ok {
		return nil, cfgr.ErrNotFound
	}
	return append([]byte(nil), contents...), nil
}

func (s *memoryContents) ReadContents(_ context.Context, document string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reads++
	contents, ok := s.data[document]
	if !ok {
		return nil, cfgr.ErrNotFound
	}
	return append([]byte(nil), contents...), nil
}

func (s *memoryContents) WriteContents(_ context.Context, document string, contents []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writes++
	s.data[document] = append([]byte(nil), contents...)
	return nil
}

type settingsRouteParams struct {
	Group string
}

func TestDefaultRouteUsesCWDJSONDocument(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := cfgr.New()
	settings := cfgr.NewRoute(cfg)

	if err := settings.WriteContents(context.Background(), []byte(`{"enabled":true}`)); err != nil {
		t.Fatal(err)
	}
	enabled, err := settings.ReadBool(context.Background(), "/enabled")
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled || string(contents) != `{"enabled":true}` {
		t.Fatalf("enabled=%v contents=%s", enabled, contents)
	}
}

func TestCustomDefaultDoesNotInitializeFilesystemStorage(t *testing.T) {
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	removedDirectory := t.TempDir()
	if err := os.Chdir(removedDirectory); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDirectory); err != nil {
			panic(err)
		}
	}()
	if err := os.RemoveAll(removedDirectory); err != nil {
		t.Fatal(err)
	}

	storage := &readOnlyContents{data: map[string][]byte{
		"config.json": []byte(`{"enabled":true}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.NewReader(storage)))
	settings := cfgr.NewRoute(cfg)

	contents, err := settings.ReadContents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"enabled":true}` {
		t.Fatalf("contents = %s", contents)
	}
	enabled, err := settings.ReadBool(context.Background(), "/enabled")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("ReadBool() = false, want true")
	}
	if err := settings.WriteContents(context.Background(), []byte(`{}`)); !errors.Is(err, cfgr.ErrUnavailable) {
		t.Fatalf("WriteContents() error = %v, want ErrUnavailable", err)
	}
	if err := settings.Write(context.Background(), "/enabled", false); !errors.Is(err, cfgr.ErrUnavailable) {
		t.Fatalf("Write() error = %v, want ErrUnavailable", err)
	}
}

func TestRouteCanAssembleParameterizedDocuments(t *testing.T) {
	storage := &memoryContents{data: map[string][]byte{
		"groups/red/settings.json": []byte("red settings"),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(storage))
	settings := cfgr.NewRouteAs[settingsRouteParams](
		cfg,
		cfgr.WithLocationBuilder(func(params settingsRouteParams) (string, error) {
			return "groups/" + params.Group + "/settings.json", nil
		}),
	)

	contents, err := settings.ReadContents(context.Background(), settingsRouteParams{Group: "red"})
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "red settings" {
		t.Fatalf("contents = %q", contents)
	}
}

func TestRouteCanPatchContentsWithoutFileAccess(t *testing.T) {
	storage := &memoryContents{data: map[string][]byte{
		"config.json": []byte("{\n  \"enabled\": false,\n  \"port\": 8080\n}\n"),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(storage)))
	settings := cfgr.NewRoute(cfg)
	patch := `*** Begin Patch
@@
-  "enabled": false,
+  "enabled": true,
*** End Patch`

	if err := settings.PatchContents(context.Background(), patch); err != nil {
		t.Fatal(err)
	}
	enabled, err := settings.ReadBool(context.Background(), "/enabled")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("ReadBool() = false, want true")
	}
}

func TestRouteCanUnsetJSONValue(t *testing.T) {
	storage := &memoryContents{data: map[string][]byte{
		"config.json": []byte(`{"enabled":true,"obsolete":"remove","port":8080}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(storage)))
	settings := cfgr.NewRoute(cfg)

	if err := settings.Unset(context.Background(), "/obsolete"); err != nil {
		t.Fatal(err)
	}
	contents, err := settings.ReadContents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"enabled":true,"port":8080}` {
		t.Fatalf("contents = %s", contents)
	}
}

func TestValueWriteAndUnsetAccessAreIndependent(t *testing.T) {
	storage := &memoryContents{data: map[string][]byte{
		"config.json": []byte(`{"enabled":false}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(storage)))
	settings := cfgr.NewRoute(
		cfg,
		cfgr.WithValueUnsetDisabled[cfgr.NoParams](),
	)

	if err := settings.Write(context.Background(), "/enabled", true); err != nil {
		t.Fatal(err)
	}
	if err := settings.Unset(context.Background(), "/enabled"); !errors.Is(err, cfgr.ErrDenied) {
		t.Fatalf("Unset() error = %v, want ErrDenied", err)
	}
}

func TestRouteCanSelectRegisteredAdapter(t *testing.T) {
	defaultStorage := &memoryContents{data: map[string][]byte{"config.json": []byte("default")}}
	stateStorage := &memoryContents{data: map[string][]byte{"config.json": []byte("registered")}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(defaultStorage))
	cfg.RegisterAdapter("state", stateStorage)
	state := cfgr.NewRoute(
		cfg,
		cfgr.WithAdapter[cfgr.NoParams]("state"),
	)

	contents, err := state.ReadContents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "registered" {
		t.Fatalf("contents = %q", contents)
	}
}

func TestRouteRejectsUnregisteredAdapter(t *testing.T) {
	cfg := cfgr.New(cfgr.WithDefaultStorage(&memoryContents{data: map[string][]byte{}}))

	defer func() {
		if recover() == nil {
			t.Fatal("NewRoute() did not reject an unregistered adapter")
		}
	}()
	cfgr.NewRoute(
		cfg,
		cfgr.WithAdapter[cfgr.NoParams]("missing"),
	)
}

func TestContentsAndValueCapabilitiesAreIndependent(t *testing.T) {
	files := &memoryContents{data: map[string][]byte{
		"config.json": []byte(`{"enabled":true}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(files)))
	settings := cfgr.NewRouteAs[settingsRouteParams](
		cfg,
		cfgr.WithContentsReadAccess(func(context.Context, settingsRouteParams) (bool, error) {
			return false, nil
		}),
		cfgr.WithValueReadAccess(func(context.Context, settingsRouteParams, string) (bool, error) {
			return true, nil
		}),
	)

	if _, err := settings.ReadContents(context.Background(), settingsRouteParams{}); !errors.Is(err, cfgr.ErrDenied) {
		t.Fatalf("ReadContents() error = %v, want ErrDenied", err)
	}
	enabled, err := settings.ReadBool(context.Background(), settingsRouteParams{}, "/enabled")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("ReadBool() = false, want true")
	}
}

func TestListFiltersValuesByTheirCapabilities(t *testing.T) {
	files := &memoryContents{data: map[string][]byte{
		"config.json": []byte(`{"public":true,"secret":"hidden"}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(files)))
	settings := cfgr.NewRouteAs[settingsRouteParams](
		cfg,
		cfgr.WithValueReadAccess(func(_ context.Context, _ settingsRouteParams, key string) (bool, error) {
			return key != "/secret", nil
		}),
	)

	entries, err := settings.List(context.Background(), settingsRouteParams{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != "/public" || entries[0].Value != true {
		t.Fatalf("visible entries = %#v", entries)
	}
}

func TestStaticRouteCanDisableWrites(t *testing.T) {
	files := &memoryContents{data: map[string][]byte{
		"config.json": []byte(`{"enabled":true}`),
	}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(jsonstore.New(files)))
	settings := cfgr.NewRoute(
		cfg,
		cfgr.WithContentsWriteDisabled[cfgr.NoParams](),
		cfgr.WithValueWriteDisabled[cfgr.NoParams](),
	)

	if err := settings.WriteContents(context.Background(), []byte(`{}`)); !errors.Is(err, cfgr.ErrDenied) {
		t.Fatalf("WriteContents() error = %v, want ErrDenied", err)
	}
	if err := settings.Write(context.Background(), "/enabled", false); !errors.Is(err, cfgr.ErrDenied) {
		t.Fatalf("Write() error = %v, want ErrDenied", err)
	}
	enabled, err := settings.ReadBool(context.Background(), "/enabled")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("ReadBool() = false, want true")
	}
}

func TestValueReadRequiresValueStorage(t *testing.T) {
	storage := &memoryContents{data: map[string][]byte{"config.json": []byte(`{}`)}}
	cfg := cfgr.New(cfgr.WithDefaultStorage(storage))
	settings := cfgr.NewRouteAs[settingsRouteParams](cfg)

	_, err := settings.Read(context.Background(), settingsRouteParams{}, "/enabled")
	if !errors.Is(err, cfgr.ErrUnavailable) {
		t.Fatalf("Read() error = %v, want ErrUnavailable", err)
	}
	if err := settings.Unset(context.Background(), settingsRouteParams{}, "/enabled"); !errors.Is(err, cfgr.ErrUnavailable) {
		t.Fatalf("Unset() error = %v, want ErrUnavailable", err)
	}
}

# go-cfgr

`go-cfgr` is a small typed configuration router. Applications own route
parameters and capability decisions. Storage adapters own serialization, key
syntax, and persistence.

## Standard configuration

```go
cfg := cfgr.New()

settings := cfgr.Register(
    cfg,
    "settings",
)
```

The default route stores JSON in:

```text
<current working directory>/settings.json
```

Complete contents and JSON Pointer values are both available:

```go
contents, err := settings.ReadContents(ctx)

err = settings.WriteContents(
    ctx,
    []byte(`{"enabled":true}`),
)

enabled, err := settings.ReadBool(
    ctx,
    "/enabled",
)

err = settings.Unset(ctx, "/obsolete")
```

Text contents can also be patched without exposing their backing file:

```go
err := settings.PatchContents(ctx, `*** Begin Patch
@@
-  "enabled": false,
+  "enabled": true,
*** End Patch`)
```

Patches are strict, pathless, and atomic in memory: every hunk must match
unambiguously before replacement contents are written. The independent
`textpatch` subpackage provides the underlying string transformation. Because
patch matching observes existing text, `PatchContents` requires both contents-read
and contents-write access.

Static routes do not expose route parameters. Use `RegisterAs[P]` when route
parameters select the document.

## Parameterized documents

`WithLocationBuilder` maps route parameters to the key understood by the selected
storage adapter. For filesystem storage that key is a relative path:

```go
type SettingsRouteParams struct {
    GroupID string
}

settings := cfgr.RegisterAs[SettingsRouteParams](
    cfg,
    "settings",
    cfgr.WithLocationBuilder(func(p SettingsRouteParams) (string, error) {
        if p.GroupID == "" {
            return "", errors.New("group ID is required")
        }
        return path.Join("groups", p.GroupID, "settings.json"), nil
    }),
)
```

## Consumer capabilities

Identity remains consumer-owned in `context.Context`. Contents and values have
separate capability hooks because they are separate storage families:

```go
settings := cfgr.RegisterAs[SettingsRouteParams](
    cfg,
    "settings",
    cfgr.WithContentsReadAccess(
        func(ctx context.Context, p SettingsRouteParams) (bool, error) {
            return permissions.CanReadSettingsContents(ctx, p.GroupID)
        },
    ),
    cfgr.WithValueWriteAccess(
        func(ctx context.Context, p SettingsRouteParams, key string) (bool, error) {
            return permissions.CanWriteSetting(ctx, p.GroupID, key)
        },
    ),
)
```

The application decides what caller identity and permissions mean. `go-cfgr`
only enforces the returned access decisions. Contents and value reads and writes
can each have their own decision function. Unset has its own key-specific access
decision through `WithValueUnsetAccess` and can be structurally disabled with
`WithValueUnsetDisabled[P]`.

Access functions run before the location builder. They must therefore tolerate
route parameters that have not yet been validated by the location builder.

A static route can be made read-only with the same generic option family:

```go
settings := cfgr.Register(
    cfg,
    "settings",
    cfgr.WithContentsWriteDisabled[cfgr.NoParams](),
    cfgr.WithValueWriteDisabled[cfgr.NoParams](),
)
```

## Alternative adapters

A router has a default adapter and can register additional adapters:

```go
cfg.RegisterAdapter(
    "state",
    databaseStorage,
)

state := cfgr.RegisterAs[StateRouteParams](
    cfg,
    "state",
    cfgr.WithAdapter[StateRouteParams]("state"),
)
```

A custom default can be supplied when creating the router:

```go
cfg := cfgr.New(
    cfgr.WithDefaultStorage(databaseStorage),
)
```

## Storage capability families

The neutral `storage` package defines small operation interfaces:

- `storage.ContentsReader` and `storage.ContentsWriter`
- `storage.ValueReader`, `storage.ValueWriter`, `storage.ValueUnsetter`, and
  `storage.ValueLister`
- `storage.TypedValues`: typed value reads

`storage.Contents` and `storage.Values` compose their respective operation
interfaces for adapters that implement the complete family. Every registered
adapter must implement `ContentsReader`; all other operations are detected when
called and return `cfgr.ErrUnavailable` when unsupported.

First-party adapters:

- `storage/fs`: contained atomic filesystem content storage
- `storage/json`: JSON Pointer value access over content storage; `NewReader`
  preserves read-only storage while `New` composes reading and writing
- `storage/jsondir`: read-only recursive merging of JSON object files from a
  directory in lexical filename order

The `jsondoc` package beneath the JSON adapters records object key order while
parsing. Existing keys retain their positions when values are updated; new keys
append to their containing object. JSON may be reformatted, but unrelated maps
are not reordered. Duplicate object keys are rejected as ambiguous configuration.

## JSON configuration directories

Individual fragments can use an ordinary writable JSON route while a second
read-only route uses `storage/jsondir` to expose their merged contents:

```go
files, err := fs.New(root)
mergedDirectory, err := jsondir.New(root)

cfg := cfgr.New(cfgr.WithDefaultStorage(json.New(files)))
cfg.RegisterAdapter("merged-directory", json.NewReader(mergedDirectory))

fragments := cfgr.RegisterAs[FragmentRouteParams](
    cfg,
    "config-fragment",
    cfgr.WithLocationBuilder(fragmentLocation), // config.d/<name>.json
)

config := cfgr.Register(
    cfg,
    "config",
    cfgr.WithAdapter[cfgr.NoParams]("merged-directory"),
    cfgr.WithLocationBuilder(func(cfgr.NoParams) (string, error) {
        return "config.d", nil
    }),
    cfgr.WithContentsWriteDisabled[cfgr.NoParams](),
)
```

Fragment location builders remain responsible for validating consumer-defined
fragment names. Merging requires object roots, recursively merges objects, and
replaces arrays, scalars, and `null` with the value from the later file. Only
regular `*.json` files are accepted as layers.

The default router composes JSON value access over filesystem contents rooted at
the current working directory. JSON value writes, unsets, and text patches use
read-modify-write; filesystem atomic replacement prevents torn files but does not
prevent concurrent lost updates.

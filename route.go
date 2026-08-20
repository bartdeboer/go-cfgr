package cfgr

import (
	"context"
	"fmt"

	"github.com/bartdeboer/go-cfgr/textpatch"
)

type LocationBuilder[P any] func(P) (string, error)

type ContentsAccess[P any] func(context.Context, P) (bool, error)
type ValueAccess[P any] func(context.Context, P, string) (bool, error)

type RouteOption[P any] func(*Route[P])

func WithLocationBuilder[P any](build LocationBuilder[P]) RouteOption[P] {
	if build == nil {
		panic("cfgr: document assembler is required")
	}
	return func(route *Route[P]) {
		route.buildLocation = build
	}
}

func WithContentsReadAccess[P any](access ContentsAccess[P]) RouteOption[P] {
	return func(route *Route[P]) {
		if access != nil {
			route.contentsReadAccess = access
		}
	}
}

func WithContentsWriteAccess[P any](access ContentsAccess[P]) RouteOption[P] {
	return func(route *Route[P]) {
		if access != nil {
			route.contentsWriteAccess = access
		}
	}
}

func WithValueReadAccess[P any](access ValueAccess[P]) RouteOption[P] {
	return func(route *Route[P]) {
		if access != nil {
			route.valueReadAccess = access
		}
	}
}

func WithValueWriteAccess[P any](access ValueAccess[P]) RouteOption[P] {
	return func(route *Route[P]) {
		if access != nil {
			route.valueWriteAccess = access
		}
	}
}

func WithValueUnsetAccess[P any](access ValueAccess[P]) RouteOption[P] {
	return func(route *Route[P]) {
		if access != nil {
			route.valueUnsetAccess = access
		}
	}
}

func WithContentsWriteDisabled[P any]() RouteOption[P] {
	return WithContentsWriteAccess(func(context.Context, P) (bool, error) {
		return false, nil
	})
}

func WithValueWriteDisabled[P any]() RouteOption[P] {
	return WithValueWriteAccess(func(context.Context, P, string) (bool, error) {
		return false, nil
	})
}

func WithValueUnsetDisabled[P any]() RouteOption[P] {
	return WithValueUnsetAccess(func(context.Context, P, string) (bool, error) {
		return false, nil
	})
}

func WithAdapter[P any](identifier string) RouteOption[P] {
	if identifier == "" {
		panic("cfgr: adapter identifier is required")
	}
	return func(route *Route[P]) {
		route.adapterIdentifier = identifier
	}
}

type Route[P any] struct {
	name                string
	adapterIdentifier   string
	adapter             adapter
	buildLocation       LocationBuilder[P]
	contentsReadAccess  ContentsAccess[P]
	contentsWriteAccess ContentsAccess[P]
	valueReadAccess     ValueAccess[P]
	valueWriteAccess    ValueAccess[P]
	valueUnsetAccess    ValueAccess[P]
}

// RegisterAs binds a named parameterized route to cfg and returns its typed
// handle. With no options it uses the default adapter, <name>.json, and allows
// all access.
func RegisterAs[P any](cfg *Router, name string, options ...RouteOption[P]) *Route[P] {
	if cfg == nil {
		panic("cfgr: router is required")
	}
	if name == "" {
		panic("cfgr: route name is required")
	}

	route := &Route[P]{
		name:              name,
		adapterIdentifier: cfg.defaultAdapter.name,
		buildLocation: func(P) (string, error) {
			return name + ".json", nil
		},
		contentsReadAccess: func(context.Context, P) (bool, error) {
			return true, nil
		},
		contentsWriteAccess: func(context.Context, P) (bool, error) {
			return true, nil
		},
		valueReadAccess: func(context.Context, P, string) (bool, error) {
			return true, nil
		},
		valueWriteAccess: func(context.Context, P, string) (bool, error) {
			return true, nil
		},
		valueUnsetAccess: func(context.Context, P, string) (bool, error) {
			return true, nil
		},
	}
	for _, option := range options {
		if option != nil {
			option(route)
		}
	}
	adapter, exists := cfg.adapters[route.adapterIdentifier]
	if !exists {
		panic(fmt.Sprintf("cfgr: adapter %q is not registered", route.adapterIdentifier))
	}
	route.adapter = adapter
	return route
}

func (r *Route[P]) Name() string { return r.name }

func (r *Route[P]) ReadContents(ctx context.Context, params P) ([]byte, error) {
	allowed, err := r.contentsReadAccess(ctx, params)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrDenied
	}
	document, err := r.location(params)
	if err != nil {
		return nil, err
	}
	contents, err := r.adapter.storage.ReadContents(ctx, document)
	return contents, r.wrap("read contents", err)
}

func (r *Route[P]) WriteContents(ctx context.Context, params P, contents []byte) error {
	allowed, err := r.contentsWriteAccess(ctx, params)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	storage, ok := r.adapter.storage.(ContentsWriter)
	if !ok {
		return ErrUnavailable
	}
	document, err := r.location(params)
	if err != nil {
		return err
	}
	return r.wrap("write contents", storage.WriteContents(ctx, document, contents))
}

// PatchContents applies a pathless text patch to the current contents and
// writes the complete result. It requires contents-write access and storage.
func (r *Route[P]) PatchContents(ctx context.Context, params P, patch string) error {
	allowed, err := r.contentsReadAccess(ctx, params)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	allowed, err = r.contentsWriteAccess(ctx, params)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	storage, ok := r.adapter.storage.(ContentsWriter)
	if !ok {
		return ErrUnavailable
	}
	location, err := r.location(params)
	if err != nil {
		return err
	}
	contents, err := r.adapter.storage.ReadContents(ctx, location)
	if err != nil {
		return r.wrap("read contents for patch", err)
	}
	updated, err := textpatch.Apply(string(contents), patch)
	if err != nil {
		return r.wrap("patch contents", err)
	}
	return r.wrap("write patched contents", storage.WriteContents(ctx, location, []byte(updated)))
}

func (r *Route[P]) Read(ctx context.Context, params P, key string) (any, error) {
	allowed, err := r.valueReadAccess(ctx, params, key)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrDenied
	}
	storage, ok := r.adapter.storage.(ValueReader)
	if !ok {
		return nil, ErrUnavailable
	}
	document, err := r.location(params)
	if err != nil {
		return nil, err
	}
	value, err := storage.Read(ctx, document, key)
	return value, r.wrap("read value", err)
}

func (r *Route[P]) Write(ctx context.Context, params P, key string, value any) error {
	allowed, err := r.valueWriteAccess(ctx, params, key)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	storage, ok := r.adapter.storage.(ValueWriter)
	if !ok {
		return ErrUnavailable
	}
	document, err := r.location(params)
	if err != nil {
		return err
	}
	return r.wrap("write value", storage.Write(ctx, document, key, value))
}

// Unset removes a value from its document. Its exact semantics are defined by
// the selected storage adapter; it does not delete the complete document.
func (r *Route[P]) Unset(ctx context.Context, params P, key string) error {
	allowed, err := r.valueUnsetAccess(ctx, params, key)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	storage, ok := r.adapter.storage.(ValueUnsetter)
	if !ok {
		return ErrUnavailable
	}
	document, err := r.location(params)
	if err != nil {
		return err
	}
	return r.wrap("unset value", storage.Unset(ctx, document, key))
}

func (r *Route[P]) List(ctx context.Context, params P, key string) ([]Entry, error) {
	allowed, err := r.valueReadAccess(ctx, params, key)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrDenied
	}
	storage, ok := r.adapter.storage.(ValueLister)
	if !ok {
		return nil, ErrUnavailable
	}
	document, err := r.location(params)
	if err != nil {
		return nil, err
	}
	entries, err := storage.List(ctx, document, key)
	if err != nil {
		return nil, r.wrap("list values", err)
	}

	visible := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		allowed, err := r.valueReadAccess(ctx, params, entry.Key)
		if err != nil {
			return nil, err
		}
		if allowed {
			visible = append(visible, entry)
		}
	}
	return visible, nil
}

func (r *Route[P]) ReadString(ctx context.Context, params P, key string) (string, error) {
	storage, document, err := r.typedTarget(ctx, params, key)
	if err != nil {
		return "", err
	}
	value, err := storage.ReadString(ctx, document, key)
	return value, r.wrap("read string", err)
}

func (r *Route[P]) ReadBool(ctx context.Context, params P, key string) (bool, error) {
	storage, document, err := r.typedTarget(ctx, params, key)
	if err != nil {
		return false, err
	}
	value, err := storage.ReadBool(ctx, document, key)
	return value, r.wrap("read bool", err)
}

func (r *Route[P]) ReadInt(ctx context.Context, params P, key string) (int, error) {
	storage, document, err := r.typedTarget(ctx, params, key)
	if err != nil {
		return 0, err
	}
	value, err := storage.ReadInt(ctx, document, key)
	return value, r.wrap("read int", err)
}

func (r *Route[P]) ReadFloat(ctx context.Context, params P, key string) (float64, error) {
	storage, document, err := r.typedTarget(ctx, params, key)
	if err != nil {
		return 0, err
	}
	value, err := storage.ReadFloat(ctx, document, key)
	return value, r.wrap("read float", err)
}

func (r *Route[P]) ReadInto(ctx context.Context, params P, key string, dst any) error {
	storage, document, err := r.typedTarget(ctx, params, key)
	if err != nil {
		return err
	}
	return r.wrap("read typed value", storage.ReadInto(ctx, document, key, dst))
}

func (r *Route[P]) typedTarget(ctx context.Context, params P, key string) (TypedValueStorage, string, error) {
	allowed, err := r.valueReadAccess(ctx, params, key)
	if err != nil {
		return nil, "", err
	}
	if !allowed {
		return nil, "", ErrDenied
	}
	storage, ok := r.adapter.storage.(TypedValueStorage)
	if !ok {
		return nil, "", ErrUnavailable
	}
	document, err := r.location(params)
	return storage, document, err
}

func (r *Route[P]) location(params P) (string, error) {
	location, err := r.buildLocation(params)
	if err != nil {
		return "", fmt.Errorf("build route %q location: %w", r.name, err)
	}
	if location == "" {
		return "", fmt.Errorf("build route %q location: empty location", r.name)
	}
	return location, nil
}

func (r *Route[P]) wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s for route %q: %w", operation, r.name, err)
}

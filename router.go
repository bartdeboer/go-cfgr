package cfgr

import (
	"fmt"

	fsstore "github.com/bartdeboer/go-cfgr/storage/fs"
	jsonstore "github.com/bartdeboer/go-cfgr/storage/json"
)

type adapter struct {
	name    string
	storage ContentsReader
}

// Router owns the default and named storage adapters used by routes.
type Router struct {
	defaultAdapter adapter
	adapters       map[string]adapter
}

type RouterOption func(*Router)

// WithDefaultStorage replaces the default JSON-over-filesystem adapter.
func WithDefaultStorage(storage ContentsReader) RouterOption {
	if storage == nil {
		panic("cfgr: default storage is required")
	}
	return func(router *Router) {
		router.defaultAdapter = adapter{name: "default", storage: storage}
		router.adapters["default"] = router.defaultAdapter
	}
}

// New creates a router whose default adapter stores JSON documents in the
// current working directory.
func New(options ...RouterOption) *Router {
	router := &Router{adapters: make(map[string]adapter)}
	for _, option := range options {
		if option != nil {
			option(router)
		}
	}
	if router.defaultAdapter.storage == nil {
		files, err := fsstore.New(".")
		if err != nil {
			panic(fmt.Errorf("cfgr: initialize default filesystem storage: %w", err))
		}
		router.defaultAdapter = adapter{name: "default", storage: jsonstore.New(files)}
		router.adapters["default"] = router.defaultAdapter
	}
	return router
}

// RegisterAdapter adds storage that routes can select by name.
func (r *Router) RegisterAdapter(identifier string, storage ContentsReader) {
	if r == nil {
		panic("cfgr: router is required")
	}
	if identifier == "" {
		panic("cfgr: adapter identifier is required")
	}
	if storage == nil {
		panic("cfgr: adapter storage is required")
	}
	if _, exists := r.adapters[identifier]; exists {
		panic(fmt.Sprintf("cfgr: adapter %q is already registered", identifier))
	}
	r.adapters[identifier] = adapter{name: identifier, storage: storage}
}

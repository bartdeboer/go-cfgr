// Package storage defines the capability interfaces implemented by cfgr storage adapters.
package storage

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("cfgr: not found")

type ContentsReader interface {
	ReadContents(ctx context.Context, document string) ([]byte, error)
}

type ContentsWriter interface {
	WriteContents(ctx context.Context, document string, contents []byte) error
}

type Contents interface {
	ContentsReader
	ContentsWriter
}

type Entry struct {
	Key   string
	Value any
}

type ValueReader interface {
	Read(ctx context.Context, document, key string) (any, error)
}

type ValueWriter interface {
	Write(ctx context.Context, document, key string, value any) error
}

type ValueUnsetter interface {
	Unset(ctx context.Context, document, key string) error
}

type ValueLister interface {
	List(ctx context.Context, document, key string) ([]Entry, error)
}

type Values interface {
	ValueReader
	ValueWriter
	ValueUnsetter
	ValueLister
}

type TypedValues interface {
	ReadString(ctx context.Context, document, key string) (string, error)
	ReadBool(ctx context.Context, document, key string) (bool, error)
	ReadInt(ctx context.Context, document, key string) (int, error)
	ReadFloat(ctx context.Context, document, key string) (float64, error)
	ReadInto(ctx context.Context, document, key string, dst any) error
}

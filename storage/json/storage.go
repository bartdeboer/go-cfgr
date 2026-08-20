// Package json adds JSON Pointer value access to document storage.
package json

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bartdeboer/go-cfgr/storage"
)

type Reader struct {
	contents storage.ContentsReader
}

func NewReader(contents storage.ContentsReader) *Reader {
	if contents == nil {
		panic("cfgr/json: contents reader is required")
	}
	return &Reader{contents: contents}
}

type Storage struct {
	*Reader
	contents storage.Contents
}

func New(contents storage.Contents) *Storage {
	if contents == nil {
		panic("cfgr/json: contents storage is required")
	}
	return &Storage{Reader: NewReader(contents), contents: contents}
}

func (s *Reader) ReadContents(ctx context.Context, document string) ([]byte, error) {
	return s.contents.ReadContents(ctx, document)
}

func (s *Storage) WriteContents(ctx context.Context, document string, contents []byte) error {
	return s.contents.WriteContents(ctx, document, contents)
}

func (s *Reader) Read(ctx context.Context, document, key string) (any, error) {
	doc, err := s.document(ctx, document)
	if err != nil {
		return nil, err
	}
	return doc.Get(key)
}

func (s *Storage) Write(ctx context.Context, document, key string, value any) error {
	doc, err := s.document(ctx, document)
	if err != nil {
		return err
	}
	if err := doc.Set(key, value); err != nil {
		return err
	}
	contents, err := doc.Marshal()
	if err != nil {
		return err
	}
	return s.contents.WriteContents(ctx, document, contents)
}

func (s *Storage) Unset(ctx context.Context, document, key string) error {
	doc, err := s.document(ctx, document)
	if err != nil {
		return err
	}
	if err := doc.Unset(key); err != nil {
		return err
	}
	contents, err := doc.Marshal()
	if err != nil {
		return err
	}
	return s.contents.WriteContents(ctx, document, contents)
}

func (s *Reader) List(ctx context.Context, document, key string) ([]storage.Entry, error) {
	doc, err := s.document(ctx, document)
	if err != nil {
		return nil, err
	}
	value, err := doc.Get(key)
	if err != nil {
		return nil, err
	}

	switch value := value.(type) {
	case map[string]any:
		keys, err := doc.OrderedKeys(key)
		if err != nil {
			return nil, err
		}
		entries := make([]storage.Entry, 0, len(keys))
		for _, child := range keys {
			entries = append(entries, storage.Entry{Key: joinPointer(key, child), Value: value[child]})
		}
		return entries, nil
	case []any:
		entries := make([]storage.Entry, 0, len(value))
		for index, item := range value {
			entries = append(entries, storage.Entry{Key: joinPointer(key, strconv.Itoa(index)), Value: item})
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("cfgr/json: value at %q cannot be listed", key)
	}
}

func (s *Reader) ReadString(ctx context.Context, document, key string) (string, error) {
	value, err := s.Read(ctx, document, key)
	if err != nil {
		return "", err
	}
	result, ok := value.(string)
	if !ok {
		return "", typeError(key, "string", value)
	}
	return result, nil
}

func (s *Reader) ReadBool(ctx context.Context, document, key string) (bool, error) {
	value, err := s.Read(ctx, document, key)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, typeError(key, "bool", value)
	}
	return result, nil
}

func (s *Reader) ReadInt(ctx context.Context, document, key string) (int, error) {
	value, err := s.Read(ctx, document, key)
	if err != nil {
		return 0, err
	}
	number, ok := value.(stdjson.Number)
	if !ok {
		return 0, typeError(key, "int", value)
	}
	result, err := strconv.Atoi(number.String())
	if err != nil {
		return 0, typeError(key, "int", value)
	}
	return result, nil
}

func (s *Reader) ReadFloat(ctx context.Context, document, key string) (float64, error) {
	value, err := s.Read(ctx, document, key)
	if err != nil {
		return 0, err
	}
	number, ok := value.(stdjson.Number)
	if !ok {
		return 0, typeError(key, "float64", value)
	}
	result, err := number.Float64()
	if err != nil {
		return 0, typeError(key, "float64", value)
	}
	return result, nil
}

func (s *Reader) ReadInto(ctx context.Context, document, key string, dst any) error {
	if dst == nil {
		return errors.New("cfgr/json: destination is required")
	}
	value, err := s.Read(ctx, document, key)
	if err != nil {
		return err
	}
	data, err := stdjson.Marshal(value)
	if err != nil {
		return err
	}
	if err := stdjson.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("cfgr/json: decode %q: %w", key, err)
	}
	return nil
}

func (s *Reader) document(ctx context.Context, document string) (*Document, error) {
	contents, err := s.contents.ReadContents(ctx, document)
	if err != nil {
		return nil, err
	}
	return NewDocument(contents)
}

func joinPointer(parent, child string) string {
	child = strings.ReplaceAll(strings.ReplaceAll(child, "~", "~0"), "/", "~1")
	if parent == "" {
		return "/" + child
	}
	return parent + "/" + child
}

func typeError(key, want string, value any) error {
	return fmt.Errorf("cfgr/json: value at %q is %T, not %s", key, value, want)
}

var (
	_ storage.ContentsReader = (*Reader)(nil)
	_ storage.ValueReader    = (*Reader)(nil)
	_ storage.ValueLister    = (*Reader)(nil)
	_ storage.TypedValues    = (*Reader)(nil)
	_ storage.Contents       = (*Storage)(nil)
	_ storage.Values         = (*Storage)(nil)
)

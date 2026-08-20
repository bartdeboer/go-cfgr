// Package jsondir merges a directory of JSON object documents.
package jsondir

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bartdeboer/go-cfgr/jsondoc"
	"github.com/bartdeboer/go-cfgr/storage"
)

type Storage struct{ root string }

func New(root string) (*Storage, error) {
	if root == "" {
		return nil, errors.New("cfgr/jsondir: root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cfgr/jsondir: resolve root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("cfgr/jsondir: create root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("cfgr/jsondir: resolve root symlinks: %w", err)
	}
	return &Storage{root: resolved}, nil
}

// ReadContents reads *.json files from location in lexical filename order and
// recursively overlays their object contents.
func (s *Storage) ReadContents(ctx context.Context, location string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory, missing, err := s.safeDirectory(location)
	if err != nil {
		return nil, err
	}
	if missing {
		return []byte("{}"), nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("cfgr/jsondir: read %q: %w", location, err)
	}

	documents := make([]*jsondoc.Document, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("cfgr/jsondir: symlink layer is not allowed: %q", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("cfgr/jsondir: inspect layer %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("cfgr/jsondir: layer is not a regular file: %q", entry.Name())
		}
		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("cfgr/jsondir: read layer %q: %w", entry.Name(), err)
		}
		document, err := jsondoc.Parse(contents)
		if err != nil {
			return nil, fmt.Errorf("cfgr/jsondir: invalid layer %q: %w", entry.Name(), err)
		}
		if !document.IsObject() {
			return nil, fmt.Errorf("cfgr/jsondir: invalid layer %q: root must be an object", entry.Name())
		}
		documents = append(documents, document)
	}

	merged, err := jsondoc.Merge(documents...)
	if err != nil {
		return nil, err
	}
	return merged.Marshal()
}

func (s *Storage) safeDirectory(location string) (string, bool, error) {
	if location == "" || filepath.IsAbs(location) {
		return "", false, fmt.Errorf("cfgr/jsondir: invalid location %q", location)
	}
	clean := filepath.Clean(location)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("cfgr/jsondir: location escapes root: %q", location)
	}
	directory := filepath.Join(s.root, clean)
	relative, _ := filepath.Rel(s.root, directory)
	current := s.root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return directory, true, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("cfgr/jsondir: inspect %q: %w", location, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false, fmt.Errorf("cfgr/jsondir: symlink component is not allowed: %q", part)
		}
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("cfgr/jsondir: location is not a directory: %q", location)
	}
	return directory, false, nil
}

var _ storage.ContentsReader = (*Storage)(nil)

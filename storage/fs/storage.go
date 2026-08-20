package fs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bartdeboer/go-cfgr/storage"
)

type Storage struct{ root string }

func New(root string) (*Storage, error) {
	if root == "" {
		return nil, errors.New("config/fs: root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("config/fs: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("config/fs: create root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("config/fs: resolve root symlinks: %w", err)
	}
	return &Storage{root: resolved}, nil
}

func (s *Storage) ReadContents(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.safePath(key, false)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("config/fs: read %q: %w", key, err)
	}
	return data, nil
}

func (s *Storage) WriteContents(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.safePath(key, true)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config/fs: create parent: %w", err)
	}
	// Recheck after creating parents so a pre-existing symlink cannot be hidden.
	if _, err := s.safePath(key, false); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".go-cfgr-*")
	if err != nil {
		return fmt.Errorf("config/fs: create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return fmt.Errorf("config/fs: write temporary file: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("config/fs: close temporary file: %w", closeErr)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("config/fs: replace %q: %w", key, err)
	}
	return nil
}

func (s *Storage) safePath(key string, allowMissing bool) (string, error) {
	if key == "" || filepath.IsAbs(key) {
		return "", fmt.Errorf("config/fs: invalid key %q", key)
	}
	clean := filepath.Clean(key)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config/fs: key escapes root: %q", key)
	}
	path := filepath.Join(s.root, clean)

	rel, _ := filepath.Rel(s.root, path)
	current := s.root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if allowMissing {
				return path, nil
			}
			return "", storage.ErrNotFound
		}
		if err != nil {
			return "", fmt.Errorf("config/fs: inspect key %q: %w", key, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("config/fs: symlink component is not allowed: %q", strings.Join(parts[:i+1], "/"))
		}
	}
	return path, nil
}

var _ storage.Contents = (*Storage)(nil)

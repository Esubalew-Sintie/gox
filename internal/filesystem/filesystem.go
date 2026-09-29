package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/constants/errors"
)

type FileSystem struct{}

func New() *FileSystem {
	return &FileSystem{}
}

func (fs *FileSystem) CreateDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (fs *FileSystem) CreateFile(path string, content []byte) error {
	if fs.Exists(path) {
		return errors.ErrPathExists
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, content, 0644)
}

func (fs *FileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile replaces an existing file through a temporary sibling. A failed
// write therefore cannot leave a partially written generated source file.
func (fs *FileSystem) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".gox-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}

	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}

	return nil
}

func (fs *FileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (fs *FileSystem) Remove(path string) error {
	if !fs.Exists(path) {
		return errors.ErrPathNotFound
	}

	return os.RemoveAll(path)
}

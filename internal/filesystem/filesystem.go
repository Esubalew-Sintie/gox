package filesystem

import (
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

func (fs *FileSystem) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, content, 0644)
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

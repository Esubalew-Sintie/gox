package injector

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	temp, err := os.CreateTemp(
		dir,
		".gox-*",
	)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	tempPath := temp.Name()

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()

		return fmt.Errorf("write temporary file: %w", err)
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
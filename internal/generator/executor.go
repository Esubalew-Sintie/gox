package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/filesystem"
)

type Executor struct {
	fs *filesystem.FileSystem
}

func NewExecutor(fs *filesystem.FileSystem) *Executor {
	return &Executor{fs: fs}
}

func (e *Executor) Execute(projectPath string, plan *GenerationPlan) error {
	if plan == nil {
		return errors.New("generation plan is nil")
	}
	if plan.Empty() {
		return nil
	}
	if err := e.Preflight(projectPath, plan); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	backups, err := e.backupModifiedFiles(projectPath, plan)
	if err != nil {
		return fmt.Errorf("backup plan files: %w", err)
	}

	completed := make([]FileOperation, 0, plan.Len())
	for _, operation := range plan.Operations {
		if err := e.executeOperation(projectPath, operation); err != nil {
			if rollbackErr := e.rollback(projectPath, completed, backups); rollbackErr != nil {
				return fmt.Errorf("execute %s %q: %w; rollback: %v", operation.Type, operation.Path, err, rollbackErr)
			}
			return fmt.Errorf("execute %s %q: %w", operation.Type, operation.Path, err)
		}
		completed = append(completed, operation)
	}
	return nil
}

func (e *Executor) backupModifiedFiles(projectPath string, plan *GenerationPlan) (map[string][]byte, error) {
	backups := make(map[string][]byte)
	for _, operation := range plan.Operations {
		if operation.Type != OperationModify && operation.Type != OperationDelete {
			continue
		}
		path := filepath.Join(projectPath, operation.Path)
		content, err := e.fs.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", operation.Path, err)
		}
		backups[operation.Path] = content
	}
	return backups, nil
}

func (e *Executor) rollback(projectPath string, completed []FileOperation, backups map[string][]byte) error {
	var rollbackErrors []error
	for index := len(completed) - 1; index >= 0; index-- {
		operation := completed[index]
		path := filepath.Join(projectPath, operation.Path)

		switch operation.Type {
		case OperationCreate:
			if err := e.fs.Remove(path); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
		case OperationModify, OperationDelete:
			if err := e.fs.WriteFile(path, backups[operation.Path]); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
	}
	return errors.Join(rollbackErrors...)
}

func (e *Executor) Preflight(projectPath string, plan *GenerationPlan) error {
	if plan == nil {
		return errors.New("generation plan is nil")
	}
	if !e.fs.Exists(projectPath) {
		return fmt.Errorf("project path does not exist: %s", projectPath)
	}

	for _, operation := range plan.Operations {
		path := filepath.Join(projectPath, operation.Path)
		if !isWithinProject(projectPath, path) {
			return fmt.Errorf("path escapes project directory: %s", operation.Path)
		}

		switch operation.Type {
		case OperationCreate:
			if e.fs.Exists(path) {
				return fmt.Errorf("file already exists: %s", operation.Path)
			}
		case OperationModify, OperationDelete:
			if !e.fs.Exists(path) {
				return fmt.Errorf("file does not exist: %s", operation.Path)
			}
		default:
			return fmt.Errorf("unsupported operation: %q", operation.Type)
		}
	}
	return nil
}

func (e *Executor) executeOperation(projectPath string, operation FileOperation) error {
	path := filepath.Join(projectPath, operation.Path)
	switch operation.Type {
	case OperationCreate:
		return e.fs.CreateFile(path, operation.Content)
	case OperationModify:
		return e.fs.WriteFile(path, operation.Content)
	case OperationDelete:
		return e.fs.Remove(path)
	default:
		return fmt.Errorf("unsupported operation: %q", operation.Type)
	}
}

func isWithinProject(projectPath string, targetPath string) bool {
	projectAbs, err := filepath.Abs(projectPath)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(projectAbs, targetAbs)
	if err != nil {
		return false
	}
	return relative != ".." && len(relative) >= 3 && relative[:3] != ".."+string(os.PathSeparator)
}

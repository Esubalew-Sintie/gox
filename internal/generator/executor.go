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

func NewExecutor(
	fs *filesystem.FileSystem,
) *Executor {
	return &Executor{
		fs: fs,
	}
}

func (e *Executor) Execute(
	projectPath string,
	plan *GenerationPlan,
) error {
	if plan == nil {
		return errors.New("generation plan is nil")
	}

	if plan.Empty() {
		return nil
	}

	if err := e.Preflight(projectPath, plan); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	for _, operation := range plan.Operations {
		if err := e.executeOperation(
			projectPath,
			operation,
		); err != nil {
			return fmt.Errorf(
				"execute %s %q: %w",
				operation.Type,
				operation.Path,
				err,
			)
		}
	}

	return nil
}

func (e *Executor) Preflight(
	projectPath string,
	plan *GenerationPlan,
) error {
	if plan == nil {
		return errors.New("generation plan is nil")
	}

	if !e.fs.Exists(projectPath) {
		return fmt.Errorf(
			"project path does not exist: %s",
			projectPath,
		)
	}

	for _, operation := range plan.Operations {
		path := filepath.Join(
			projectPath,
			operation.Path,
		)

		if !isWithinProject(
			projectPath,
			path,
		) {
			return fmt.Errorf(
				"path escapes project directory: %s",
				operation.Path,
			)
		}

		switch operation.Type {
		case OperationCreate:
			if e.fs.Exists(path) {
				return fmt.Errorf(
					"file already exists: %s",
					operation.Path,
				)
			}

		case OperationModify:
			if !e.fs.Exists(path) {
				return fmt.Errorf(
					"file does not exist: %s",
					operation.Path,
				)
			}

		case OperationDelete:
			if !e.fs.Exists(path) {
				return fmt.Errorf(
					"file does not exist: %s",
					operation.Path,
				)
			}

		default:
			return fmt.Errorf(
				"unsupported operation: %q",
				operation.Type,
			)
		}
	}

	return nil
}

func (e *Executor) executeOperation(
	projectPath string,
	operation FileOperation,
) error {
	path := filepath.Join(
		projectPath,
		operation.Path,
	)

	switch operation.Type {
	case OperationCreate:
		return e.fs.CreateFile(
			path,
			operation.Content,
		)

	case OperationModify:
		return e.fs.WriteFile(
			path,
			operation.Content,
		)

	case OperationDelete:
		return e.fs.Remove(path)

	default:
		return fmt.Errorf(
			"unsupported operation: %q",
			operation.Type,
		)
	}
}

func isWithinProject(
	projectPath string,
	targetPath string,
) bool {
	projectAbs, err := filepath.Abs(projectPath)
	if err != nil {
		return false
	}

	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	relative, err := filepath.Rel(
		projectAbs,
		targetAbs,
	)
	if err != nil {
		return false
	}

	return relative != ".." &&
		len(relative) >= 3 &&
		relative[:3] != ".."+string(os.PathSeparator)
}
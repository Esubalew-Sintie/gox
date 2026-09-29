package generator

import (
	"fmt"
	"strings"
)

type OperationType string

const (
	OperationCreate OperationType = "create"
	OperationModify OperationType = "modify"
	OperationDelete OperationType = "delete"
)

type FileOperation struct {
	Type    OperationType
	Path    string
	Content []byte
}

type GenerationPlan struct {
	Operations []FileOperation
}

func NewGenerationPlan() *GenerationPlan {
	return &GenerationPlan{
		Operations: make([]FileOperation, 0),
	}
}

func (p *GenerationPlan) AddCreate(
	path string,
	content []byte,
) error {
	return p.add(FileOperation{
		Type:    OperationCreate,
		Path:    path,
		Content: content,
	})
}

func (p *GenerationPlan) AddModify(
	path string,
	content []byte,
) error {
	return p.add(FileOperation{
		Type:    OperationModify,
		Path:    path,
		Content: content,
	})
}

func (p *GenerationPlan) AddDelete(
	path string,
) error {
	return p.add(FileOperation{
		Type: OperationDelete,
		Path: path,
	})
}

func (p *GenerationPlan) add(
	operation FileOperation,
) error {
	if strings.TrimSpace(operation.Path) == "" {
		return fmt.Errorf("operation path is required")
	}

	for _, existing := range p.Operations {
		if existing.Path == operation.Path {
			return fmt.Errorf(
				"duplicate operation for path %q",
				operation.Path,
			)
		}
	}

	p.Operations = append(
		p.Operations,
		operation,
	)

	return nil
}

func (p *GenerationPlan) Empty() bool {
	return len(p.Operations) == 0
}

func (p *GenerationPlan) Len() int {
	return len(p.Operations)
}
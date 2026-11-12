package codemod

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
)

func printNode(buf *bytes.Buffer, fset *token.FileSet, node any) error {
	if err := format.Node(buf, fset, node); err != nil {
		return fmt.Errorf("print source: %w", err)
	}
	return nil
}

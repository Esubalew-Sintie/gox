package injector

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

type Injector struct{}

func New() *Injector {
	return &Injector{}
}

func (i *Injector) Inject(source []byte, marker Marker, content string) ([]byte, error) {
	if strings.TrimSpace(content) == "" {
		return source, nil
	}
	if marker == ImportsMarker {
		return injectImport(source, content)
	}

	start := []byte(marker.Start())
	end := []byte(marker.End())
	startIndex := bytes.Index(source, start)
	if startIndex == -1 {
		return nil, fmt.Errorf("marker %q not found", marker.Start())
	}

	endIndex := bytes.Index(source[startIndex+len(start):], end)
	if endIndex == -1 {
		return nil, fmt.Errorf("marker %q not found", marker.End())
	}
	endIndex += startIndex + len(start)

	regionStart := startIndex + len(start)
	region := source[regionStart:endIndex]
	if containsInjectedCode(region, content) {
		return source, nil
	}

	var result bytes.Buffer
	result.Write(source[:regionStart])
	result.WriteByte('\n')
	result.WriteString(strings.TrimSpace(content))
	result.WriteByte('\n')
	if existing := bytes.TrimSpace(region); len(existing) > 0 {
		result.Write(existing)
		result.WriteByte('\n')
	}
	result.Write(source[endIndex:])

	formatted, err := format.Source(result.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format injected source: %w", err)
	}
	return formatted, nil
}

func injectImport(source []byte, content string) ([]byte, error) {
	path, err := strconv.Unquote(strings.TrimSpace(content))
	if err != nil {
		return nil, fmt.Errorf("parse import %q: %w", content, err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse source for import injection: %w", err)
	}

	for _, imported := range file.Imports {
		value, _ := strconv.Unquote(imported.Path.Value)
		if value == path {
			return source, nil
		}
	}

	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		gen.Lparen = gen.Pos()
		gen.Specs = append(gen.Specs, spec)
		return formatAST(fileSet, file)
	}

	file.Decls = append([]ast.Decl{&ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: token.Pos(1),
		Specs:  []ast.Spec{spec},
	}}, file.Decls...)
	return formatAST(fileSet, file)
}

func formatAST(fileSet *token.FileSet, file *ast.File) ([]byte, error) {
	var output bytes.Buffer
	if err := format.Node(&output, fileSet, file); err != nil {
		return nil, fmt.Errorf("format import injection: %w", err)
	}
	return output.Bytes(), nil
}

func containsInjectedCode(region []byte, content string) bool {
	return bytes.Contains(region, []byte(strings.TrimSpace(content)))
}

func (i *Injector) Remove(source []byte, marker Marker, content string) ([]byte, error) {
	if strings.TrimSpace(content) == "" {
		return source, nil
	}
	if marker == ImportsMarker {
		return removeImport(source, content)
	}

	start := []byte(marker.Start())
	end := []byte(marker.End())
	startIndex := bytes.Index(source, start)
	if startIndex == -1 {
		return nil, fmt.Errorf("marker %q not found", marker.Start())
	}
	endIndex := bytes.Index(source[startIndex+len(start):], end)
	if endIndex == -1 {
		return nil, fmt.Errorf("marker %q not found", marker.End())
	}
	endIndex += startIndex + len(start)

	regionStart := startIndex + len(start)
	region := source[regionStart:endIndex]
	needle := []byte(strings.TrimSpace(content))
	position := bytes.Index(region, needle)
	if position == -1 {
		return source, nil
	}

	lineStart := position
	for lineStart > 0 && region[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := position + len(needle)
	for lineEnd < len(region) && region[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd < len(region) {
		lineEnd++
	}

	var result bytes.Buffer
	result.Write(source[:regionStart])
	result.Write(region[:lineStart])
	result.Write(region[lineEnd:])
	result.Write(source[endIndex:])

	formatted, err := format.Source(result.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format removed source: %w", err)
	}
	return formatted, nil
}

func removeImport(source []byte, content string) ([]byte, error) {
	path, err := strconv.Unquote(strings.TrimSpace(content))
	if err != nil {
		return nil, fmt.Errorf("parse import %q: %w", content, err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse source for import removal: %w", err)
	}

	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for index, spec := range gen.Specs {
			imported := spec.(*ast.ImportSpec)
			value, _ := strconv.Unquote(imported.Path.Value)
			if value != path {
				continue
			}
			if importIsUsed(file, importIdentifier(path)) {
				return source, nil
			}
			gen.Specs = append(gen.Specs[:index], gen.Specs[index+1:]...)
			return formatAST(fileSet, file)
		}
	}
	return source, nil
}

func importIdentifier(importPath string) string {
	parts := strings.Split(strings.TrimSuffix(importPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	name := parts[len(parts)-1]
	if len(name) > 1 && name[0] == 'v' {
		digits := true
		for _, character := range name[1:] {
			if character < '0' || character > '9' {
				digits = false
				break
			}
		}
		if digits && len(parts) > 1 {
			name = parts[len(parts)-2]
		}
	}
	return name
}

func importIsUsed(file *ast.File, name string) bool {
	if name == "" {
		return false
	}

	used := false
	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == name {
			used = true
			return false
		}
		return !used
	})
	return used
}

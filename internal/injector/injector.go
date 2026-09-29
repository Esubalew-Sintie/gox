package injector

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
)

type Injector struct{}

func New() *Injector {
	return &Injector{}
}

func (i *Injector) Inject(
	source []byte,
	marker Marker,
	content string,
) ([]byte, error) {
	if strings.TrimSpace(content) == "" {
		return source, nil
	}

	start := []byte(marker.Start())
	end := []byte(marker.End())

	startIndex := bytes.Index(source, start)

	if startIndex == -1 {
		return nil, fmt.Errorf(
			"marker %q not found",
			marker.Start(),
		)
	}

	endIndex := bytes.Index(
		source[startIndex+len(start):],
		end,
	)

	if endIndex == -1 {
		return nil, fmt.Errorf(
			"marker %q not found",
			marker.End(),
		)
	}

	endIndex += startIndex + len(start)

	regionStart := startIndex + len(start)
	region := source[regionStart:endIndex]

	if containsInjectedCode(region, content) {
		return source, nil
	}

	content = strings.TrimSpace(content)

	var result bytes.Buffer

	result.Write(source[:regionStart])

	// Always ensure a newline after the start marker before injected content.
	result.WriteByte('\n')
	result.WriteString(content)
	result.WriteByte('\n')

	// Preserve any existing content in the region (after our new line).
	existing := bytes.TrimSpace(region)
	if len(existing) > 0 {
		result.Write(existing)
		result.WriteByte('\n')
	}

	result.Write(source[endIndex:])

	formatted, err := format.Source(result.Bytes())
	if err != nil {
		return nil, fmt.Errorf(
			"format injected source: %w",
			err,
		)
	}

	return formatted, nil
}

func containsInjectedCode(
	region []byte,
	content string,
) bool {
	return bytes.Contains(
		region,
		[]byte(strings.TrimSpace(content)),
	)
}
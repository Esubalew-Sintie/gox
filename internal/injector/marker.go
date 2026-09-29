package injector

import "fmt"

type Marker string

const (
	ImportsMarker Marker = "imports"
	FieldsMarker  Marker = "fields"
	InitMarker    Marker = "init"
	RoutesMarker  Marker = "routes"
)

func (m Marker) Start() string {
	return fmt.Sprintf("// gox:%s:start", m)
}

func (m Marker) End() string {
	return fmt.Sprintf("// gox:%s:end", m)
}
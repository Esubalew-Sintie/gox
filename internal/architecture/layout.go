package architecture

import "strings"

const (
	LayeredREST  = "layered-rest"
	FeatureFirst = "feature-first"
)

func Normalize(layout string) string {
	switch strings.TrimSpace(strings.ToLower(layout)) {
	case "", "clean-rest", LayeredREST:
		return LayeredREST
	case FeatureFirst:
		return FeatureFirst
	default:
		return ""
	}
}

func IsSupported(layout string) bool {
	return Normalize(layout) != ""
}

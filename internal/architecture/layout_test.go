package architecture

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"":              LayeredREST,
		"clean-rest":    LayeredREST,
		"layered-rest":  LayeredREST,
		"feature-first": FeatureFirst,
		"unknown":       "",
	}

	for input, want := range tests {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

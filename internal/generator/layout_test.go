package generator

import (
	"testing"

	"github.com/esubalew-Sintie/gox/internal/architecture"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

func TestBuildModuleLayout(t *testing.T) {
	name, err := naming.Parse("account profile")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		layout    string
		firstPath string
	}{
		{architecture.LayeredREST, "internal/storage/persistence/account_profile/account_profile.go"},
		{architecture.FeatureFirst, "internal/feature/account_profile/repository.go"},
	}

	for _, test := range tests {
		spec, err := buildModuleLayout(test.layout, "example.com/app", name)
		if err != nil {
			t.Fatalf("buildModuleLayout(%q): %v", test.layout, err)
		}
		if got := spec.files[0].path; got != test.firstPath {
			t.Errorf("%s first path = %q, want %q", test.layout, got, test.firstPath)
		}
		wantWiring := 4
		if test.layout == architecture.LayeredREST {
			wantWiring = 7
		}
		if len(spec.wiring) != wantWiring {
			t.Errorf("%s wiring count = %d, want %d", test.layout, len(spec.wiring), wantWiring)
		}
	}
}

package release

import "testing"

func TestDefaultCatalogMappings(t *testing.T) {
	t.Parallel()

	repo, ok := UpstreamRepo("pac")
	if !ok || repo != "tektoncd/pipelines-as-code" {
		t.Fatalf("UpstreamRepo(pac) = %q, %v", repo, ok)
	}

	if got, want := DisplayName("pac"), "Pipelines as Code"; got != want {
		t.Fatalf("DisplayName(pac) = %q, want %q", got, want)
	}

	names := UpstreamComponentNames()
	if len(names) == 0 || names[0] != "chains" {
		t.Fatalf("unexpected component names: %#v", names)
	}
}

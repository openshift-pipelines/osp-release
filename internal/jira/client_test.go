package jira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolverResolve(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "testdata", "jira_versions.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var payload []Version
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	versions := make([]string, 0, len(payload))
	for _, version := range payload {
		versions = append(versions, version.Name)
	}

	resolver := Resolver{versions: versions}

	if got, want := resolver.Resolve("1.21"), "1.21.2"; got != want {
		t.Fatalf("Resolve(1.21) = %q, want %q", got, want)
	}
	if got, want := resolver.Resolve("1.22"), "1.22.0"; got != want {
		t.Fatalf("Resolve(1.22) = %q, want %q", got, want)
	}
	if got := resolver.Resolve("9.99"); got != "" {
		t.Fatalf("Resolve(9.99) = %q, want empty", got)
	}
}

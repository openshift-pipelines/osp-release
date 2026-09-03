package app

import "testing"

func TestExitCodesAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[int]string{}
	for _, code := range ExitCodes() {
		if name, ok := seen[code.Code]; ok {
			t.Fatalf("exit code %d used by both %q and %q", code.Code, name, code.Name)
		}
		seen[code.Code] = code.Name
		if code.Meaning == "" {
			t.Fatalf("exit code %q has no meaning", code.Name)
		}
	}
}

func TestKindsAreUniqueAndMapToKnownExitCodes(t *testing.T) {
	t.Parallel()

	valid := map[int]bool{}
	for _, code := range ExitCodes() {
		valid[code.Code] = true
	}

	seen := map[string]bool{}
	for _, kind := range Kinds() {
		if kind.Name == "" {
			t.Fatal("kind with empty name")
		}
		if seen[kind.Name] {
			t.Fatalf("duplicate kind %q", kind.Name)
		}
		seen[kind.Name] = true
		if kind.Meaning == "" {
			t.Fatalf("kind %q has no meaning", kind.Name)
		}
		if !valid[kind.Exit] {
			t.Fatalf("kind %q maps to unknown exit code %d", kind.Name, kind.Exit)
		}
	}
}

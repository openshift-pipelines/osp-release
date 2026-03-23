package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestResolveEnvAndFlagOverrides(t *testing.T) {
	t.Parallel()

	resolver := NewResolverWithRunner(func(_ context.Context, name string, args ...string) (string, error) {
		if name != "pass" || len(args) != 2 || args[0] != "show" {
			t.Fatalf("unexpected runner call: %s %v", name, args)
		}
		switch args[1] {
		case "jira/email":
			return "user@example.com\n", nil
		case "jira/token":
			return "secret-token\n", nil
		default:
			return "", errors.New("unknown entry")
		}
	})

	creds, err := resolver.Resolve(
		context.Background(),
		"pass::jira/email",
		"env-token",
		"",
		"pass::jira/token",
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if creds.Email != "user@example.com" {
		t.Fatalf("creds.Email = %q", creds.Email)
	}
	if creds.Token != "secret-token" {
		t.Fatalf("creds.Token = %q", creds.Token)
	}
}

func TestResolveRequiresCredentials(t *testing.T) {
	t.Parallel()

	resolver := NewResolverWithRunner(func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", nil
	})

	if _, err := resolver.Resolve(context.Background(), "", "", "", ""); err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
}

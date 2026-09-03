package credentials

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift-pipelines/osp-release/internal/app"
)

const passPrefix = "pass::"

type Credentials struct {
	Email string
	Token string
}

type Runner func(context.Context, string, ...string) (string, error)

type Resolver struct {
	run Runner
}

func NewResolver() Resolver {
	return Resolver{run: defaultRunner}
}

func NewResolverWithRunner(run Runner) Resolver {
	return Resolver{run: run}
}

func (r Resolver) Resolve(ctx context.Context, envEmail, envToken, flagEmail, flagToken string) (Credentials, error) {
	email := strings.TrimSpace(envEmail)
	token := strings.TrimSpace(envToken)

	if strings.TrimSpace(flagEmail) != "" {
		email = strings.TrimSpace(flagEmail)
	}
	if strings.TrimSpace(flagToken) != "" {
		token = strings.TrimSpace(flagToken)
	}

	resolvedEmail, err := r.resolveValue(ctx, email, "jira email")
	if err != nil {
		return Credentials{}, err
	}
	resolvedToken, err := r.resolveValue(ctx, token, "jira token")
	if err != nil {
		return Credentials{}, err
	}

	if resolvedEmail == "" || resolvedToken == "" {
		return Credentials{}, app.New(
			app.KindMissingCredentials,
			"missing Jira credentials; set OSP_JIRA_EMAIL and OSP_JIRA_TOKEN or pass --jira-email/--jira-token",
			app.ExitAuth,
			nil,
			nil,
		)
	}

	return Credentials{
		Email: resolvedEmail,
		Token: resolvedToken,
	}, nil
}

func (r Resolver) resolveValue(ctx context.Context, value, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, passPrefix) {
		return value, nil
	}

	entry := strings.TrimSpace(strings.TrimPrefix(value, passPrefix))
	if entry == "" {
		return "", app.New(
			app.KindInvalidPassReference,
			fmt.Sprintf("%s pass reference is empty", label),
			app.ExitUsage,
			map[string]any{"field": label},
			nil,
		)
	}

	output, err := r.run(ctx, "pass", "show", entry)
	if err != nil {
		return "", app.New(
			app.KindPassLookupFailed,
			fmt.Sprintf("failed to resolve %s from pass entry %q", label, entry),
			app.ExitDependency,
			map[string]any{"field": label, "entry": entry},
			err,
		)
	}

	return strings.TrimSpace(output), nil
}

func defaultRunner(ctx context.Context, name string, args ...string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}

	return stdout.String(), nil
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	apperr "github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/release"
	"github.com/openshift-pipelines/osp-release/internal/upstream"
	"github.com/openshift-pipelines/osp-release/internal/version"
)

func describeApp(stdout *bytes.Buffer, isTTY bool) application {
	return application{
		streams:        streams{out: stdout, err: &bytes.Buffer{}, isTTY: isTTY},
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{records: map[string]string{}}),
	}
}

func runDescribe(t *testing.T, isTTY bool, args ...string) *bytes.Buffer {
	t.Helper()

	var stdout bytes.Buffer
	command := newRootCommand(context.Background(), describeApp(&stdout, isTTY))
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	return &stdout
}

// TestDescribeCoversEveryCommand is the anti-drift guard: a new runnable
// command with no entry in commandMetadata fails here.
func TestDescribeCoversEveryCommand(t *testing.T) {
	t.Parallel()

	root := newRootCommand(context.Background(), describeApp(&bytes.Buffer{}, false))
	metadata := commandMetadata()

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			if child.Runnable() {
				path := commandPath(root, child)
				if _, ok := metadata[path]; !ok {
					t.Errorf("command %q has no commandMetadata entry", path)
				}
			}
			walk(child)
		}
	}
	walk(root)

	described := collectCommands(root, metadata)
	for path := range metadata {
		if !slices.ContainsFunc(described, func(command describeCommand) bool { return command.Path == path }) {
			t.Errorf("commandMetadata has entry %q with no matching command", path)
		}
	}
}

func TestDescribeDefaultsToJSONOnTTY(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, true, "describe")

	var schema describeSchema
	if err := json.Unmarshal(stdout.Bytes(), &schema); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if schema.Binary != "osp-release" {
		t.Fatalf("Binary = %q", schema.Binary)
	}
	if schema.Version == "" {
		t.Fatal("Version is empty")
	}
	if len(schema.ExitCodes) != len(apperr.ExitCodes()) {
		t.Fatalf("len(ExitCodes) = %d, want %d", len(schema.ExitCodes), len(apperr.ExitCodes()))
	}
	if len(schema.ErrorKinds) == 0 {
		t.Fatal("ErrorKinds is empty")
	}
	if schema.Cache.TTLDays != 7 {
		t.Fatalf("Cache.TTLDays = %d, want 7", schema.Cache.TTLDays)
	}
}

func TestDescribeTableOutputIsAvailable(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, false, "describe", "commands", "--output", "table")
	if !strings.Contains(stdout.String(), "release show") {
		t.Fatalf("table output missing commands:\n%s", stdout.String())
	}
}

func TestDescribeFieldsMatchRenderer(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, false, "describe", "fields", "release", "show")

	var payload struct {
		Path    string   `json:"path"`
		Allowed []string `json:"allowed"`
		Default []string `json:"default"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if payload.Path != "release show" {
		t.Fatalf("Path = %q", payload.Path)
	}
	if !slices.Equal(payload.Allowed, release.AllowedReleaseFields()) {
		t.Fatalf("Allowed = %v, want %v", payload.Allowed, release.AllowedReleaseFields())
	}
	if !slices.Equal(payload.Default, release.DefaultReleaseFields()) {
		t.Fatalf("Default = %v, want %v", payload.Default, release.DefaultReleaseFields())
	}
}

func TestDescribeCommandReportsArgsAndAuth(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, false, "describe", "commands", "release", "show")

	var command describeCommand
	if err := json.Unmarshal(stdout.Bytes(), &command); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(command.Args) != 1 || command.Args[0].Name != "minor" || !command.Args[0].Required {
		t.Fatalf("Args = %#v", command.Args)
	}
	if !slices.Equal(command.Auth, jiraAuth) {
		t.Fatalf("Auth = %v, want %v", command.Auth, jiraAuth)
	}
	if !command.Cached {
		t.Fatal("Cached = false, want true")
	}
	if !slices.ContainsFunc(command.Flags, func(flag describeFlag) bool { return flag.Name == "field" }) {
		t.Fatalf("Flags missing inherited --field: %#v", command.Flags)
	}
}

func TestDescribeUnknownCommandExitsNotFound(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	command := newRootCommand(context.Background(), describeApp(&stdout, false))
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetArgs([]string{"describe", "commands", "nope"})

	err := command.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	appErr, ok := apperr.Extract(err)
	if !ok {
		t.Fatalf("Extract() = false for %v", err)
	}
	if appErr.Kind != apperr.KindUnknownCommand || appErr.Exit != apperr.ExitNotFound {
		t.Fatalf("Kind = %q, Exit = %d", appErr.Kind, appErr.Exit)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, false, "version")

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	for _, field := range version.Fields() {
		if _, ok := payload[field]; !ok {
			t.Fatalf("payload missing %q: %#v", field, payload)
		}
	}
	if payload["version"] == "" {
		t.Fatal("version is empty")
	}
}

func TestVersionCommandQuietField(t *testing.T) {
	t.Parallel()

	stdout := runDescribe(t, true, "version", "--field", "version", "--quiet")
	if strings.TrimSpace(stdout.String()) != version.Get().Version {
		t.Fatalf("stdout = %q, want %q", stdout.String(), version.Get().Version)
	}
}

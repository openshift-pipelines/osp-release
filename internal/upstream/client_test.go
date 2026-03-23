package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type countingRESTClient struct {
	calls    map[string]int
	payloads map[string]any
}

func (c *countingRESTClient) Get(path string, out any) error {
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[path]++

	payload, ok := c.payloads[path]
	if !ok {
		return errors.New("404 not found")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func TestResolveComponentVersionUsesSeriesAndCache(t *testing.T) {
	t.Parallel()

	rest := &countingRESTClient{
		payloads: map[string]any{
			"repos/tektoncd/pipelines-as-code/releases?per_page=100": []releaseResponse{
				{TagName: "v0.39.0"},
				{TagName: "v0.39.2"},
				{TagName: "v0.40.1"},
				{TagName: "v0.39.1-rc1", Prerelease: true},
			},
		},
	}
	client := NewClientWithRESTClient(rest)

	if got := client.ResolveComponentVersion(context.TODO(), "pac", "0.39.x"); got != "0.39.2" {
		t.Fatalf("ResolveComponentVersion() = %q", got)
	}
	if got := client.ResolveComponentVersion(context.TODO(), "pac", "0.39.x"); got != "0.39.2" {
		t.Fatalf("ResolveComponentVersion() second call = %q", got)
	}
	if rest.calls["repos/tektoncd/pipelines-as-code/releases?per_page=100"] != 1 {
		t.Fatalf("expected cache hit, got %d calls", rest.calls["repos/tektoncd/pipelines-as-code/releases?per_page=100"])
	}
}

func TestResolveComponentVersionFallsBackToRawValue(t *testing.T) {
	t.Parallel()

	client := NewClientWithRESTClient(&countingRESTClient{
		payloads: map[string]any{},
	})

	if got := client.ResolveComponentVersion(context.TODO(), "pac", "0.39.x"); got != "0.39.x" {
		t.Fatalf("ResolveComponentVersion() = %q", got)
	}
	if got := client.ResolveComponentVersion(context.TODO(), "pac", "0.39.2"); got != "0.39.2" {
		t.Fatalf("ResolveComponentVersion() exact = %q", got)
	}
}

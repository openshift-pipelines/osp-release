package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	apperr "github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/credentials"
	"github.com/openshift-pipelines/osp-release/internal/release"
	"github.com/openshift-pipelines/osp-release/internal/upstream"
)

type fakeRESTClient struct {
	records map[string]string
}

func (f fakeRESTClient) Get(path string, out any) error {
	tag, ok := f.records[path]
	if !ok {
		return errors.New("404 not found")
	}
	target, ok := out.(*struct {
		TagName string `json:"tag_name"`
	})
	if ok {
		target.TagName = tag
		return nil
	}
	b, _ := json.Marshal(map[string]string{"tag_name": tag})
	return json.Unmarshal(b, out)
}

func TestReleaseListDefaultsToJSONWhenNotTTY(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "list", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(payload) != 2 {
		t.Fatalf("len(payload) = %d", len(payload))
	}
	if payload[0]["version"] != "1.21.2" || payload[1]["version"] != "1.20.4" {
		t.Fatalf("unexpected version ordering: %#v", payload)
	}
	if payload[0]["pac"] != "0.39.x" {
		t.Fatalf("expected default components in payload: %#v", payload[0])
	}
}

func TestReleaseShowComponentsRendersSecondTable(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: true},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "show", "1.21", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered := stdout.String()
	if !strings.Contains(rendered, "Minor") || !strings.Contains(rendered, "Component") || !strings.Contains(rendered, "0.39.x") {
		t.Fatalf("unexpected output: %q", rendered)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty: %q", stderr.String())
	}
}

func TestReleaseAllDefaultsToJSONWhenNotTTY(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "all", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(payload) != 2 {
		t.Fatalf("len(payload) = %d", len(payload))
	}
	if payload[0]["minor"] != "1.21" || payload[1]["minor"] != "1.20" {
		t.Fatalf("unexpected release order: %#v", payload)
	}
}

func TestReleaseListIncludesUnreleasedWithFlag(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "list", "--unreleased", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(payload) != 3 || payload[0]["minor"] != "1.22" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestUpstreamListDefaultsToTableWhenTTY(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: true},
		httpClient: &http.Client{},
		creds: credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) {
			return "", nil
		}),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{
				"repos/tektoncd/pipelines-as-code/releases/latest": "v0.40.1",
				"repos/tektoncd/pipeline/releases/latest":          "v1.0.0",
				"repos/tektoncd/cli/releases/latest":               "v0.38.0",
				"repos/tektoncd/triggers/releases/latest":          "v0.30.0",
				"repos/tektoncd/chains/releases/latest":            "v0.24.0",
				"repos/tektoncd/results/releases/latest":           "v0.15.0",
				"repos/tektoncd/hub/releases/latest":               "v0.1.0",
				"repos/openshift-pipelines/opc/releases/latest":    "v0.10.0",
			},
		}),
		siteURL:      release.Site,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: release.LifecycleURL,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"upstream", "list"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered := strings.ToLower(stdout.String())
	if !strings.Contains(rendered, "component") || !strings.Contains(rendered, "pac") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty: %q", stderr.String())
	}
}

func TestComponentShowJSON(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{
				"repos/tektoncd/pipelines-as-code/releases/latest": "v0.40.1",
			},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"component", "show", "pac", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if payload["display_name"] != "Pipelines as Code" || payload["latest_upstream"] != "v0.40.1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	releases, ok := payload["releases"].([]any)
	if !ok || len(releases) != 3 {
		t.Fatalf("unexpected releases payload: %#v", payload["releases"])
	}
}

// countingServer wraps a mux and records how many times each path is hit.
type countingServer struct {
	mu     sync.Mutex
	counts map[string]int
	mux    http.Handler
}

func (c *countingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.counts[r.URL.Path]++
	c.mu.Unlock()
	c.mux.ServeHTTP(w, r)
}

func (c *countingServer) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[path]
}

func testCountingServer(t *testing.T) (*httptest.Server, *countingServer) {
	t.Helper()

	cs := &countingServer{counts: make(map[string]int), mux: testMux(t)}
	srv := httptest.NewServer(cs)
	t.Cleanup(srv.Close)
	return srv, cs
}

func TestCacheHitSkipsNetwork(t *testing.T) {
	t.Parallel()

	srv, counter := testCountingServer(t)
	cacheDir := t.TempDir()

	makeApp := func() application {
		return application{
			streams:    streams{out: new(bytes.Buffer), err: new(bytes.Buffer), isTTY: false},
			httpClient: srv.Client(),
			creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
			upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
				records: map[string]string{},
			}),
			siteURL:      srv.URL,
			pageID:       release.PageID,
			projectKey:   release.ProjectKey,
			lifecycleURL: srv.URL + lifecyclePath,
			cacheDir:     cacheDir,
		}
	}

	// First call: should hit the network.
	cmd1 := newRootCommand(context.Background(), makeApp())
	cmd1.SetArgs([]string{"release", "list", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	// Second call: cache is fresh, must NOT hit the network again.
	cmd2 := newRootCommand(context.Background(), makeApp())
	cmd2.SetArgs([]string{"release", "list", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second Execute: %v", err)
	}

	if got := counter.count("/wiki/rest/api/content/267160127"); got != 1 {
		t.Fatalf("expected 1 Confluence hit, got %d", got)
	}
	if got := counter.count("/rest/api/3/project/SRVKP/versions"); got != 1 {
		t.Fatalf("expected 1 Jira hit, got %d", got)
	}
}

func TestRefreshForcesLiveFetch(t *testing.T) {
	t.Parallel()

	srv, counter := testCountingServer(t)
	cacheDir := t.TempDir()

	makeApp := func() application {
		return application{
			streams:    streams{out: new(bytes.Buffer), err: new(bytes.Buffer), isTTY: false},
			httpClient: srv.Client(),
			creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
			upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
				records: map[string]string{},
			}),
			siteURL:      srv.URL,
			pageID:       release.PageID,
			projectKey:   release.ProjectKey,
			lifecycleURL: srv.URL + lifecyclePath,
			cacheDir:     cacheDir,
		}
	}

	// Prime the cache.
	cmd1 := newRootCommand(context.Background(), makeApp())
	cmd1.SetArgs([]string{"release", "list", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("prime Execute: %v", err)
	}

	// --refresh must bypass the cache.
	cmd2 := newRootCommand(context.Background(), makeApp())
	cmd2.SetArgs([]string{"release", "list", "--refresh", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("refresh Execute: %v", err)
	}

	if got := counter.count("/wiki/rest/api/content/267160127"); got != 2 {
		t.Fatalf("expected 2 Confluence hits after --refresh, got %d", got)
	}
}

func TestComponentListSharesCache(t *testing.T) {
	t.Parallel()

	srv, counter := testCountingServer(t)
	cacheDir := t.TempDir()

	makeApp := func() application {
		return application{
			streams:    streams{out: new(bytes.Buffer), err: new(bytes.Buffer), isTTY: false},
			httpClient: srv.Client(),
			creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
			upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
				records: map[string]string{},
			}),
			siteURL:      srv.URL,
			pageID:       release.PageID,
			projectKey:   release.ProjectKey,
			lifecycleURL: srv.URL + lifecyclePath,
			cacheDir:     cacheDir,
		}
	}

	// release list primes the cache.
	cmd1 := newRootCommand(context.Background(), makeApp())
	cmd1.SetArgs([]string{"release", "list", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("release list Execute: %v", err)
	}

	// component list reads from the same cache (no additional Confluence hit).
	cmd2 := newRootCommand(context.Background(), makeApp())
	cmd2.SetArgs([]string{"component", "list", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("component list Execute: %v", err)
	}

	if got := counter.count("/wiki/rest/api/content/267160127"); got != 1 {
		t.Fatalf("expected 1 Confluence hit total, got %d", got)
	}
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(testMux(t))
}

// lifecyclePath is the path segment of the Red Hat product life cycle API.
const lifecyclePath = "/product-life-cycles/api/v1/products"

func testMux(t *testing.T) *http.ServeMux {
	t.Helper()

	htmlFixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "confluence_page.html"))
	if err != nil {
		t.Fatalf("read html fixture: %v", err)
	}
	jiraFixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "jira_versions.json"))
	if err != nil {
		t.Fatalf("read jira fixture: %v", err)
	}
	lifecycleFixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "lifecycle.json"))
	if err != nil {
		t.Fatalf("read lifecycle fixture: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/rest/api/content/267160127", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"body": map[string]any{
				"storage": map[string]any{
					"value": string(htmlFixture),
				},
			},
		})
	})
	mux.HandleFunc("/rest/api/3/project/SRVKP/versions", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jiraFixture)
	})
	mux.HandleFunc(lifecyclePath, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(lifecycleFixture)
	})
	return mux
}

func supportTestApp(t *testing.T, stdout, stderr *bytes.Buffer, lifecycleURL string) application {
	t.Helper()
	return application{
		streams:    streams{out: stdout, err: stderr, isTTY: false},
		httpClient: &http.Client{},
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      release.Site,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: lifecycleURL,
		cacheDir:     t.TempDir(),
	}
}

func TestSupportListJSON(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := supportTestApp(t, &stdout, &stderr, server.URL+lifecyclePath)

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"support", "list"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	// 1.20 is end of life and must be hidden by default.
	if len(payload) != 2 {
		t.Fatalf("len(payload) = %d: %#v", len(payload), payload)
	}
	if payload[0]["minor"] != "1.22" || payload[1]["minor"] != "1.21" {
		t.Fatalf("expected supported versions newest first: %#v", payload)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty: %q", stderr.String())
	}
}

func TestSupportListAllIncludesEOL(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := supportTestApp(t, &stdout, &stderr, server.URL+lifecyclePath)

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"support", "list", "--all"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(payload) != 3 {
		t.Fatalf("len(payload) = %d: %#v", len(payload), payload)
	}
	if payload[2]["minor"] != "1.20" || payload[2]["support_status"] != "End of life" {
		t.Fatalf("expected the EOL release last: %#v", payload)
	}
}

// --all only affects list; show must still resolve end-of-life versions.
func TestSupportShowResolvesEOLVersion(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := supportTestApp(t, &stdout, &stderr, server.URL+lifecyclePath)

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"support", "show", "1.20", "--field", "support_status", "--quiet"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "End of life" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestSupportShowQuietField(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := supportTestApp(t, &stdout, &stderr, server.URL+lifecyclePath)

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"support", "show", "1.21", "--field", "eol_date", "--quiet"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "2026-07-17" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

// The support command needs no Jira credentials.
func TestSupportShowUnknownMinor(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := supportTestApp(t, &stdout, &stderr, server.URL+lifecyclePath)

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"support", "show", "9.99"})
	err := command.Execute()
	appErr, ok := apperr.Extract(err)
	if !ok {
		t.Fatalf("expected an app error, got %v", err)
	}
	if appErr.Kind != "support_not_found" || appErr.Exit != 3 {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func TestReleaseOutputIncludesSupportStatusButNotDates(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "list", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if payload[0]["support_status"] != "Full Support" {
		t.Fatalf("expected 1.21 support status: %#v", payload[0])
	}
	if _, ok := payload[0]["eol_date"]; ok {
		t.Fatalf("eol_date should not be in the default field set: %#v", payload[0])
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty: %q", stderr.String())
	}
}

func TestReleaseSelectsSupportDateField(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + lifecyclePath,
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "show", "1.21", "--field", "eol_date", "--quiet", "--jira-email", "u@example.com", "--jira-token", "tok"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "2026-07-17" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

// A failing life cycle API must not break release commands.
func TestReleaseSurvivesLifecycleFailure(t *testing.T) {
	t.Parallel()

	server := testServer(t)
	defer server.Close()

	var stdout, stderr bytes.Buffer
	app := application{
		streams:    streams{out: &stdout, err: &stderr, isTTY: false},
		httpClient: server.Client(),
		creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
		upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
			records: map[string]string{},
		}),
		siteURL:      server.URL,
		pageID:       release.PageID,
		projectKey:   release.ProjectKey,
		lifecycleURL: server.URL + "/does-not-exist",
		cacheDir:     t.TempDir(),
	}

	command := newRootCommand(context.Background(), app)
	command.SetArgs([]string{"release", "list", "--jira-email", "user@example.com", "--jira-token", "token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout.String())
	}
	if len(payload) != 2 || payload[0]["support_status"] != "" {
		t.Fatalf("expected releases with empty support status: %#v", payload)
	}
	if !strings.Contains(stderr.String(), "could not fetch support data") {
		t.Fatalf("expected a warning on stderr, got %q", stderr.String())
	}
}

func TestSupportCacheHitSkipsNetwork(t *testing.T) {
	t.Parallel()

	srv, counter := testCountingServer(t)
	cacheDir := t.TempDir()

	makeApp := func() application {
		return application{
			streams:    streams{out: new(bytes.Buffer), err: new(bytes.Buffer), isTTY: false},
			httpClient: srv.Client(),
			creds:      credentials.NewResolverWithRunner(func(context.Context, string, ...string) (string, error) { return "", nil }),
			upstreamClient: upstream.NewClientWithRESTClient(fakeRESTClient{
				records: map[string]string{},
			}),
			siteURL:      srv.URL,
			pageID:       release.PageID,
			projectKey:   release.ProjectKey,
			lifecycleURL: srv.URL + lifecyclePath,
			cacheDir:     cacheDir,
		}
	}

	for range 2 {
		cmd := newRootCommand(context.Background(), makeApp())
		cmd.SetArgs([]string{"support", "list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	}

	if got := counter.count(lifecyclePath); got != 1 {
		t.Fatalf("expected 1 life cycle hit, got %d", got)
	}

	// --refresh bypasses the cache.
	cmd := newRootCommand(context.Background(), makeApp())
	cmd.SetArgs([]string{"support", "list", "--refresh"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("refresh Execute: %v", err)
	}
	if got := counter.count(lifecyclePath); got != 2 {
		t.Fatalf("expected 2 life cycle hits after --refresh, got %d", got)
	}
}

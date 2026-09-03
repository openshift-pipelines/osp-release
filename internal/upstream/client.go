package upstream

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	ghapi "github.com/cli/go-gh/v2/pkg/api"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

type restClient interface {
	Get(string, any) error
}

type Client struct {
	restClient restClient
	mu         sync.RWMutex
	cache      map[string]string
}

type latestRelease struct {
	TagName string `json:"tag_name"`
}

type releaseResponse struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

var versionPattern = regexp.MustCompile(`(?i)(\d+\.\d+(?:\.\d+)*)`)

func NewClient() (*Client, error) {
	client, err := ghapi.NewRESTClient(ghapi.ClientOptions{
		EnableCache: true,
		CacheTTL:    6 * time.Hour,
	})
	if err != nil {
		return nil, app.New(app.KindGitHubClientFailed, "failed to initialize GitHub client", app.ExitDependency, nil, err)
	}
	return &Client{restClient: client, cache: map[string]string{}}, nil
}

func NewClientWithRESTClient(client restClient) *Client {
	return &Client{restClient: client, cache: map[string]string{}}
}

func (c *Client) LatestRelease(ctx context.Context, component string) (release.UpstreamRecord, error) {
	repo, ok := release.UpstreamRepo(component)
	if !ok {
		return release.UpstreamRecord{}, app.New(
			app.KindUnknownComponent,
			fmt.Sprintf("unknown component %q", component),
			app.ExitNotFound,
			map[string]any{"component": component},
			nil,
		)
	}

	var payload latestRelease
	path := fmt.Sprintf("repos/%s/releases/latest", repo)
	if err := c.restClient.Get(path, &payload); err != nil {
		message := strings.TrimSpace(err.Error())
		exitCode := app.ExitUpstream
		kind := app.KindGitHubRequestFailed
		if strings.Contains(strings.ToLower(message), "404") {
			exitCode = app.ExitNotFound
			kind = app.KindReleaseNotFound
		}
		if strings.Contains(strings.ToLower(message), "401") || strings.Contains(strings.ToLower(message), "403") {
			exitCode = app.ExitAuth
			kind = app.KindGitHubAuthFailed
		}
		return release.UpstreamRecord{}, app.New(
			kind,
			fmt.Sprintf("failed to fetch upstream release for %s", repo),
			exitCode,
			map[string]any{"component": component, "repo": repo},
			err,
		)
	}

	return release.UpstreamRecord{
		Component: component,
		Repo:      repo,
		Version:   payload.TagName,
	}, nil
}

func (c *Client) LatestReleases(ctx context.Context) ([]release.UpstreamRecord, error) {
	records := make([]release.UpstreamRecord, 0, len(release.UpstreamComponentNames()))
	for _, component := range release.UpstreamComponentNames() {
		record, err := c.LatestRelease(ctx, component)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (c *Client) ResolveComponentVersion(ctx context.Context, component, current string) string {
	_ = ctx
	current = strings.TrimSpace(current)
	if !strings.HasSuffix(strings.ToLower(current), ".x") {
		return current
	}

	repo, ok := release.UpstreamRepo(component)
	if !ok {
		return current
	}

	key := repo + "|" + current
	c.mu.RLock()
	if resolved, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return resolved
	}
	c.mu.RUnlock()

	resolved, err := c.resolveSeriesRelease(repo, current)
	if err != nil || resolved == "" {
		resolved = current
	}

	c.mu.Lock()
	c.cache[key] = resolved
	c.mu.Unlock()
	return resolved
}

func (c *Client) resolveSeriesRelease(repo, current string) (string, error) {
	var payload []releaseResponse
	path := fmt.Sprintf("repos/%s/releases?per_page=100", repo)
	if err := c.restClient.Get(path, &payload); err != nil {
		return "", err
	}

	targetPrefix := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(current), "v"), ".x") + "."
	best := ""
	for _, candidate := range payload {
		if candidate.Draft || candidate.Prerelease {
			continue
		}
		version := extractVersion(candidate.TagName)
		if version == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(version), targetPrefix) {
			continue
		}
		if best == "" || release.CompareVersions(version, best) > 0 {
			best = version
		}
	}
	return best, nil
}

func extractVersion(tag string) string {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(tag))
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

var _ restClient = (*ghapi.RESTClient)(nil)
var _ = http.StatusOK

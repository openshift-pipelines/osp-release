package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/credentials"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	ProjectKey string
}

type Version struct {
	Name string `json:"name"`
}

type Resolver struct {
	versions []string
}

func (c Client) FetchResolver(ctx context.Context, creds credentials.Credentials, debug io.Writer) (Resolver, error) {
	url := fmt.Sprintf("%s/rest/api/3/project/%s/versions", c.BaseURL, c.ProjectKey)
	if debug != nil {
		_, _ = fmt.Fprintf(debug, "[debug] fetching Jira versions for project %s\n", c.ProjectKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Resolver{}, app.New("request_build_failed", "failed to build Jira request", app.ExitGeneral, nil, err)
	}
	req.SetBasicAuth(creds.Email, creds.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Resolver{}, app.New("jira_request_failed", "failed to fetch Jira versions", app.ExitUpstream, nil, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Resolver{}, app.New("jira_auth_failed", "Jira authentication failed", app.ExitAuth, nil, nil)
	}
	if resp.StatusCode != http.StatusOK {
		return Resolver{}, app.New(
			"jira_request_failed",
			fmt.Sprintf("Jira request failed with status %d", resp.StatusCode),
			app.ExitUpstream,
			map[string]any{"status": resp.StatusCode},
			nil,
		)
	}

	var payload []Version
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Resolver{}, app.New("jira_decode_failed", "failed to decode Jira versions response", app.ExitGeneral, nil, err)
	}

	versions := make([]string, 0, len(payload))
	for _, version := range payload {
		versions = append(versions, strings.TrimSpace(version.Name))
	}
	return Resolver{versions: versions}, nil
}

func (r Resolver) Resolve(minor string) string {
	pattern := regexp.MustCompile(`(?i)^pipelines ` + regexp.QuoteMeta(minor) + `(\.\d+)?$`)
	versionPattern := regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)$`)

	var matches []string
	for _, version := range r.versions {
		if !pattern.MatchString(version) {
			continue
		}
		capture := versionPattern.FindStringSubmatch(version)
		if len(capture) < 2 {
			continue
		}
		matches = append(matches, capture[1])
	}
	if len(matches) == 0 {
		return ""
	}

	best := matches[0]
	for _, match := range matches[1:] {
		if release.CompareVersions(match, best) > 0 {
			best = match
		}
	}
	return best
}

package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/credentials"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	PageID     int
}

type pageResponse struct {
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

func (c Client) FetchTable(ctx context.Context, creds credentials.Credentials, debug io.Writer) (release.Table, error) {
	url := fmt.Sprintf("%s/wiki/rest/api/content/%d?expand=body.storage", c.BaseURL, c.PageID)
	if debug != nil {
		_, _ = fmt.Fprintf(debug, "[debug] fetching Confluence page %d\n", c.PageID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release.Table{}, app.New("request_build_failed", "failed to build Confluence request", app.ExitGeneral, nil, err)
	}
	req.SetBasicAuth(creds.Email, creds.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return release.Table{}, app.New("confluence_request_failed", "failed to fetch Confluence release table", app.ExitUpstream, nil, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return release.Table{}, app.New("confluence_auth_failed", "Confluence authentication failed", app.ExitAuth, nil, nil)
	}
	if resp.StatusCode != http.StatusOK {
		return release.Table{}, app.New(
			"confluence_request_failed",
			fmt.Sprintf("Confluence request failed with status %d", resp.StatusCode),
			app.ExitUpstream,
			map[string]any{"status": resp.StatusCode},
			nil,
		)
	}

	var payload pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return release.Table{}, app.New("confluence_decode_failed", "failed to decode Confluence response", app.ExitGeneral, nil, err)
	}

	return ParseTable(payload.Body.Storage.Value)
}

func ParseTable(pageHTML string) (release.Table, error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return release.Table{}, app.New("html_parse_failed", "failed to parse Confluence HTML", app.ExitGeneral, nil, err)
	}

	tableNode := firstElement(doc, "table")
	if tableNode == nil {
		return release.Table{}, app.New("table_not_found", "no release table found in Confluence page", app.ExitGeneral, nil, nil)
	}

	rowNodes := collectElements(tableNode, "tr")
	if len(rowNodes) == 0 {
		return release.Table{}, app.New("rows_not_found", "no rows found in Confluence table", app.ExitGeneral, nil, nil)
	}

	headers := elementTexts(rowNodes[0], "th")
	if len(headers) == 0 {
		return release.Table{}, app.New("headers_not_found", "no headers found in Confluence table", app.ExitGeneral, nil, nil)
	}

	normalizedHeaders := make([]string, 0, len(headers))
	for _, header := range headers {
		normalizedHeaders = append(normalizedHeaders, release.NormalizeHeader(header))
	}

	componentHeaders := make([]string, 0, len(normalizedHeaders))
	for _, header := range normalizedHeaders {
		if header == "version" || header == "minimum_k8s_version" {
			continue
		}
		componentHeaders = append(componentHeaders, header)
	}

	rows := make([]release.ReleaseRow, 0, len(rowNodes)-1)
	seen := map[string]struct{}{}
	for _, rowNode := range rowNodes[1:] {
		cells := directChildren(rowNode, "td")
		if len(cells) == 0 {
			continue
		}

		versionNode := firstElement(cells[0], "strong")
		if versionNode == nil {
			continue
		}
		minor := strings.TrimSpace(textContent(versionNode))
		parts := release.ParseMinor(minor)
		if len(parts) != 2 {
			continue
		}
		if _, ok := seen[minor]; ok {
			continue
		}
		seen[minor] = struct{}{}

		values := make(map[string]string, len(normalizedHeaders))
		filled := 0
		for idx, cell := range cells {
			if idx >= len(normalizedHeaders) {
				break
			}
			text := normalizeText(textContent(cell))
			values[normalizedHeaders[idx]] = text
			if text != "" {
				filled++
			}
		}

		rows = append(rows, release.ReleaseRow{
			Minor:   minor,
			Values:  values,
			Filled:  filled,
			RawHTML: renderNode(rowNode),
		})
	}

	release.SortRows(rows)
	return release.Table{
		Headers:    normalizedHeaders,
		Components: componentHeaders,
		Rows:       rows,
	}, nil
}

func firstElement(node *html.Node, name string) *html.Node {
	if node.Type == html.ElementNode && node.Data == name {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := firstElement(child, name); found != nil {
			return found
		}
	}
	return nil
}

func collectElements(node *html.Node, name string) []*html.Node {
	var out []*html.Node
	if node.Type == html.ElementNode && node.Data == name {
		out = append(out, node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		out = append(out, collectElements(child, name)...)
	}
	return out
}

func directChildren(node *html.Node, name string) []*html.Node {
	var out []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == name {
			out = append(out, child)
		}
	}
	return out
}

func elementTexts(node *html.Node, name string) []string {
	children := directChildren(node, name)
	values := make([]string, 0, len(children))
	for _, child := range children {
		values = append(values, normalizeText(textContent(child)))
	}
	return values
}

func textContent(node *html.Node) string {
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			out.WriteString(current.Data)
			out.WriteRune(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return out.String()
}

func renderNode(node *html.Node) string {
	var out strings.Builder
	_ = html.Render(&out, node)
	return out.String()
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

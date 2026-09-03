package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

const (
	phaseGA          = "general availability"
	phaseFullSupport = "full support"
	phaseMaintenance = "maintenance support"
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

type productsResponse struct {
	Data []productEntry `json:"data"`
}

type productEntry struct {
	Name     string         `json:"name"`
	Versions []versionEntry `json:"versions"`
}

type versionEntry struct {
	Name   string       `json:"name"`
	Type   string       `json:"type"`
	Phases []phaseEntry `json:"phases"`
}

type phaseEntry struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// FetchSupport retrieves the OpenShift Pipelines product life cycle data. The
// endpoint is public and requires no credentials.
func (c Client) FetchSupport(ctx context.Context, debug io.Writer) ([]release.SupportRecord, error) {
	if debug != nil {
		_, _ = fmt.Fprintf(debug, "[debug] fetching product life cycle data from %s\n", c.BaseURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL, nil)
	if err != nil {
		return nil, app.New(app.KindRequestBuildFailed, "failed to build life cycle request", app.ExitGeneral, nil, err)
	}
	query := req.URL.Query()
	query.Set("name", release.LifecycleProduct)
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, app.New(app.KindLifecycleRequestFailed, "failed to fetch product life cycle data", app.ExitUpstream, nil, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, app.New(
			app.KindLifecycleRequestFailed,
			fmt.Sprintf("product life cycle request failed with status %d", resp.StatusCode),
			app.ExitUpstream,
			map[string]any{"status": resp.StatusCode},
			nil,
		)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, app.New(app.KindLifecycleRequestFailed, "failed to read product life cycle response", app.ExitUpstream, nil, err)
	}
	return ParseSupport(raw)
}

// ParseSupport turns a product life cycle API payload into support records
// sorted from newest to oldest minor.
func ParseSupport(raw []byte) ([]release.SupportRecord, error) {
	var payload productsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, app.New(app.KindLifecycleDecodeFailed, "failed to decode product life cycle response", app.ExitGeneral, nil, err)
	}

	product, ok := findProduct(payload.Data)
	if !ok {
		return nil, app.New(
			app.KindLifecycleProductMissing,
			fmt.Sprintf("product %q not found in life cycle response", release.LifecycleProduct),
			app.ExitUpstream,
			map[string]any{"product": release.LifecycleProduct},
			nil,
		)
	}

	records := make([]release.SupportRecord, 0, len(product.Versions))
	for _, version := range product.Versions {
		minor := strings.TrimSpace(version.Name)
		if len(release.ParseMinor(minor)) != 2 {
			continue
		}
		phases := indexPhases(version.Phases)
		records = append(records, release.SupportRecord{
			Minor:          minor,
			SupportStatus:  strings.TrimSpace(version.Type),
			GADate:         normalizeDate(phases[phaseGA].EndDate),
			FullSupportEnd: normalizeDate(phases[phaseFullSupport].EndDate),
			EOLDate:        normalizeDate(phases[phaseMaintenance].EndDate),
		})
	}

	release.SortSupportRecords(records)
	return records, nil
}

func findProduct(entries []productEntry) (productEntry, bool) {
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.Name), release.LifecycleProduct) {
			return entry, true
		}
	}
	return productEntry{}, false
}

func indexPhases(phases []phaseEntry) map[string]phaseEntry {
	out := make(map[string]phaseEntry, len(phases))
	for _, phase := range phases {
		out[strings.ToLower(strings.TrimSpace(phase.Name))] = phase
	}
	return out
}

// normalizeDate converts an API timestamp to YYYY-MM-DD. Placeholder values
// such as "N/A" and empty strings become empty strings.
func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "n/a") {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.DateOnly)
	}
	if len(value) >= len(time.DateOnly) {
		if _, err := time.Parse(time.DateOnly, value[:len(time.DateOnly)]); err == nil {
			return value[:len(time.DateOnly)]
		}
	}
	return value
}

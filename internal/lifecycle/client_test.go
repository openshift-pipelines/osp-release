package lifecycle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "lifecycle.json"))
	if err != nil {
		t.Fatalf("read lifecycle fixture: %v", err)
	}
	return raw
}

func TestParseSupport(t *testing.T) {
	t.Parallel()

	records, err := ParseSupport(readFixture(t))
	if err != nil {
		t.Fatalf("ParseSupport() error = %v", err)
	}

	// Non-OSP products and non-numeric version names are skipped, and records
	// are sorted newest first.
	if len(records) != 3 {
		t.Fatalf("len(records) = %d: %#v", len(records), records)
	}
	if records[0].Minor != "1.22" || records[1].Minor != "1.21" || records[2].Minor != "1.20" {
		t.Fatalf("unexpected ordering: %#v", records)
	}

	want := release.SupportRecord{
		Minor:          "1.21",
		SupportStatus:  "Full Support",
		GADate:         "2026-01-22",
		FullSupportEnd: "2026-05-27",
		EOLDate:        "2026-07-17",
	}
	if records[1] != want {
		t.Fatalf("records[1] = %#v, want %#v", records[1], want)
	}
}

func TestParseSupportHandlesMissingPhasesAndPlaceholders(t *testing.T) {
	t.Parallel()

	records, err := ParseSupport(readFixture(t))
	if err != nil {
		t.Fatalf("ParseSupport() error = %v", err)
	}

	record, ok := release.FindSupportByMinor(records, "1.22")
	if !ok {
		t.Fatal("expected a record for 1.22")
	}
	if record.SupportStatus != "Maintenance Support" {
		t.Fatalf("SupportStatus = %q", record.SupportStatus)
	}
	if record.GADate != "" {
		t.Fatalf(`GADate = %q, want "" for the "N/A" placeholder`, record.GADate)
	}
	if record.FullSupportEnd != "" {
		t.Fatalf(`FullSupportEnd = %q, want "" for the missing phase`, record.FullSupportEnd)
	}
	if record.EOLDate != "2026-09-30" {
		t.Fatalf("EOLDate = %q", record.EOLDate)
	}
}

func TestParseSupportErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		kind string
	}{
		{name: "invalid json", raw: "{", kind: "lifecycle_decode_failed"},
		{name: "product missing", raw: `{"data":[{"name":"Other","versions":[]}]}`, kind: "lifecycle_product_not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSupport([]byte(test.raw))
			appErr, ok := app.Extract(err)
			if !ok {
				t.Fatalf("expected an app error, got %v", err)
			}
			if appErr.Kind != test.kind {
				t.Fatalf("Kind = %q, want %q", appErr.Kind, test.kind)
			}
		})
	}
}

func TestFetchSupport(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t)
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("name")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := Client{HTTPClient: server.Client(), BaseURL: server.URL}
	records, err := client.FetchSupport(context.Background(), nil)
	if err != nil {
		t.Fatalf("FetchSupport() error = %v", err)
	}
	if gotQuery != release.LifecycleProduct {
		t.Fatalf("name query = %q, want %q", gotQuery, release.LifecycleProduct)
	}
	if len(records) != 3 {
		t.Fatalf("len(records) = %d", len(records))
	}
}

func TestFetchSupportHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := Client{HTTPClient: server.Client(), BaseURL: server.URL}
	_, err := client.FetchSupport(context.Background(), nil)
	appErr, ok := app.Extract(err)
	if !ok {
		t.Fatalf("expected an app error, got %v", err)
	}
	if appErr.Kind != "lifecycle_request_failed" {
		t.Fatalf("Kind = %q", appErr.Kind)
	}
	if appErr.Exit != app.ExitUpstream {
		t.Fatalf("Exit = %d, want %d", appErr.Exit, app.ExitUpstream)
	}
}

func TestNormalizeDate(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"2026-01-22T00:00:00.000Z": "2026-01-22",
		"2026-01-22":               "2026-01-22",
		"N/A":                      "",
		"":                         "",
		"  ":                       "",
		"unknown":                  "unknown",
	}
	for input, want := range tests {
		if got := normalizeDate(input); got != want {
			t.Fatalf("normalizeDate(%q) = %q, want %q", input, got, want)
		}
	}
}

package release

import (
	"slices"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	if got := CompareVersions("1.21", "1.20"); got <= 0 {
		t.Fatalf("CompareVersions() = %d", got)
	}
	if got := CompareVersions("1.21.2", "1.21.10"); got >= 0 {
		t.Fatalf("CompareVersions() = %d", got)
	}
}

func TestBuildRecordsNewestFirst(t *testing.T) {
	t.Parallel()

	table := Table{
		Components: []string{"pac"},
		Rows: []ReleaseRow{
			{Minor: "1.20", Values: map[string]string{"pac": "0.37.x"}, Filled: 8},
			{Minor: "1.21", Values: map[string]string{"pac": "0.39.x"}, Filled: 8},
			{Minor: "1.22", Values: map[string]string{"pac": "0.40.x"}, Filled: 2},
		},
	}

	records := BuildRecords(table, map[string]string{
		"1.20": "1.20.4",
		"1.21": "1.21.2",
		"1.22": "1.22.0",
	})

	if records[0].Minor != "1.22" || records[2].Minor != "1.20" {
		t.Fatalf("unexpected order: %#v", records)
	}
	if latest, ok := LatestReleased(records); !ok || latest.Minor != "1.21" {
		t.Fatalf("LatestReleased() = %#v, %v", latest, ok)
	}
	if latest, ok := LatestUnreleased(records); !ok || latest.Minor != "1.22" {
		t.Fatalf("LatestUnreleased() = %#v, %v", latest, ok)
	}
}

func TestAttachSupport(t *testing.T) {
	t.Parallel()

	records := []ReleaseRecord{
		{Minor: "1.22"},
		{Minor: "1.21"},
		{Minor: "1.20"},
	}
	AttachSupport(records, []SupportRecord{
		{Minor: "1.21", SupportStatus: "Full Support", EOLDate: "2026-07-17"},
		{Minor: "1.19", SupportStatus: "End of life"},
	})

	if records[1].Support.SupportStatus != "Full Support" || records[1].Support.EOLDate != "2026-07-17" {
		t.Fatalf("expected 1.21 to be enriched: %#v", records[1].Support)
	}
	if records[0].Support != (SupportRecord{}) || records[2].Support != (SupportRecord{}) {
		t.Fatalf("expected unmatched records to keep an empty support record: %#v", records)
	}
}

func TestAttachSupportNoData(t *testing.T) {
	t.Parallel()

	records := []ReleaseRecord{{Minor: "1.21", Support: SupportRecord{SupportStatus: "Full Support"}}}
	AttachSupport(records, nil)
	if records[0].Support.SupportStatus != "Full Support" {
		t.Fatalf("expected existing support data to survive: %#v", records[0].Support)
	}
}

func TestReleaseRecordToMapIncludesSupportFields(t *testing.T) {
	t.Parallel()

	record := ReleaseRecord{
		Minor:      "1.21",
		Version:    "1.21.2",
		Released:   true,
		Components: map[string]string{"pac": "0.39.x"},
		Support: SupportRecord{
			Minor:          "1.21",
			SupportStatus:  "Full Support",
			GADate:         "2026-01-22",
			FullSupportEnd: "2026-05-27",
			EOLDate:        "2026-07-17",
		},
	}

	fields := record.ToMap()
	for key, want := range map[string]any{
		"support_status":   "Full Support",
		"ga_date":          "2026-01-22",
		"full_support_end": "2026-05-27",
		"eol_date":         "2026-07-17",
	} {
		if fields[key] != want {
			t.Fatalf("ToMap()[%q] = %v, want %v", key, fields[key], want)
		}
	}
}

func TestReleaseFieldSets(t *testing.T) {
	t.Parallel()

	allowed := AllowedReleaseFields()
	for _, field := range SupportFields() {
		if !slices.Contains(allowed, field) {
			t.Fatalf("AllowedReleaseFields() missing %q", field)
		}
	}

	if !slices.Contains(DefaultReleaseFields(), "support_status") {
		t.Fatalf("DefaultReleaseFields() = %v, want support_status", DefaultReleaseFields())
	}
	if slices.Contains(DefaultReleaseFields(), "eol_date") {
		t.Fatalf("DefaultReleaseFields() = %v, should not include eol_date", DefaultReleaseFields())
	}

	withComponents := ReleaseFieldsWithComponents()
	if !slices.Contains(withComponents, "support_status") || !slices.Contains(withComponents, "pac") {
		t.Fatalf("ReleaseFieldsWithComponents() = %v", withComponents)
	}
	if slices.Contains(withComponents, "eol_date") {
		t.Fatalf("ReleaseFieldsWithComponents() should not include eol_date: %v", withComponents)
	}
}

func TestSortSupportRecordsNewestFirst(t *testing.T) {
	t.Parallel()

	records := []SupportRecord{{Minor: "1.9"}, {Minor: "1.21"}, {Minor: "1.10"}}
	SortSupportRecords(records)
	if records[0].Minor != "1.21" || records[1].Minor != "1.10" || records[2].Minor != "1.9" {
		t.Fatalf("unexpected order: %#v", records)
	}
}

func TestFindSupportByMinor(t *testing.T) {
	t.Parallel()

	records := []SupportRecord{{Minor: "1.21", SupportStatus: "Full Support"}}
	if record, ok := FindSupportByMinor(records, "1.21"); !ok || record.SupportStatus != "Full Support" {
		t.Fatalf("FindSupportByMinor(1.21) = %#v, %v", record, ok)
	}
	if _, ok := FindSupportByMinor(records, "1.99"); ok {
		t.Fatal("expected no match for 1.99")
	}
}

func TestIsSupported(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"Full Support":        true,
		"Maintenance Support": true,
		"end of life":         false,
		"End of life":         false,
		"":                    true,
	}
	for status, want := range tests {
		if got := (SupportRecord{SupportStatus: status}).IsSupported(); got != want {
			t.Fatalf("IsSupported(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestFilterSupported(t *testing.T) {
	t.Parallel()

	records := []SupportRecord{
		{Minor: "1.22", SupportStatus: "Full Support"},
		{Minor: "1.21", SupportStatus: "End of life"},
		{Minor: "1.15", SupportStatus: "Maintenance Support"},
	}
	filtered := FilterSupported(records)
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d: %#v", len(filtered), filtered)
	}
	if filtered[0].Minor != "1.22" || filtered[1].Minor != "1.15" {
		t.Fatalf("unexpected filtered records: %#v", filtered)
	}
	if len(records) != 3 {
		t.Fatalf("FilterSupported must not mutate its input: %#v", records)
	}
}

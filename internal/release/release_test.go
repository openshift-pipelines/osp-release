package release

import "testing"

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

package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openshift-pipelines/osp-release/internal/release"
)

func samplePayload(fetchedAt time.Time) Payload {
	return Payload{
		FetchedAt: fetchedAt,
		Table: release.Table{
			Headers:    []string{"minor", "pac"},
			Components: []string{"pac"},
			Rows: []release.ReleaseRow{
				{Minor: "1.21", Values: map[string]string{"pac": "0.39.x"}, Filled: 8},
			},
		},
		Resolved: map[string]string{"1.21": "1.21.2"},
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	_, fresh, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Fatal("expected fresh=false for missing file")
	}
}

func TestLoadFreshPayload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	p := samplePayload(time.Now())
	if err := Save(path, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, fresh, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !fresh {
		t.Fatal("expected fresh=true for recent payload")
	}
	if got.Resolved["1.21"] != "1.21.2" {
		t.Fatalf("unexpected resolved: %v", got.Resolved)
	}
}

func TestLoadExpiredPayload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	p := samplePayload(time.Now().Add(-8 * 24 * time.Hour))
	if err := Save(path, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, fresh, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if fresh {
		t.Fatal("expected fresh=false for expired payload")
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, samplePayload(time.Now())); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Overwrite with garbage.
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestSaveAtomic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	p := samplePayload(time.Now())
	if err := Save(path, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, fresh, err := Load(path)
	if err != nil || !fresh {
		t.Fatalf("Load after Save: fresh=%v err=%v", fresh, err)
	}
	if len(got.Table.Rows) != 1 || got.Table.Rows[0].Minor != "1.21" {
		t.Fatalf("unexpected table: %v", got.Table)
	}
}

func sampleSupport() []release.SupportRecord {
	return []release.SupportRecord{
		{
			Minor:          "1.21",
			SupportStatus:  "Full Support",
			GADate:         "2026-01-22",
			FullSupportEnd: "2026-05-27",
			EOLDate:        "2026-07-17",
		},
	}
}

func TestSupportFreshness(t *testing.T) {
	t.Parallel()

	if (Payload{}).SupportFresh() {
		t.Fatal("expected SupportFresh()=false for a zero timestamp")
	}
	if !(Payload{SupportFetchedAt: time.Now()}).SupportFresh() {
		t.Fatal("expected SupportFresh()=true for a recent timestamp")
	}
	if (Payload{SupportFetchedAt: time.Now().Add(-8 * 24 * time.Hour)}).SupportFresh() {
		t.Fatal("expected SupportFresh()=false for an expired timestamp")
	}
}

func TestSaveSupportPreservesReleaseData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")

	p := samplePayload(time.Now())
	if err := SaveRelease(path, p.Table, p.Resolved); err != nil {
		t.Fatalf("SaveRelease: %v", err)
	}
	if err := SaveSupport(path, sampleSupport()); err != nil {
		t.Fatalf("SaveSupport: %v", err)
	}

	got, fresh, err := Load(path)
	if err != nil || !fresh {
		t.Fatalf("Load: fresh=%v err=%v", fresh, err)
	}
	if got.Resolved["1.21"] != "1.21.2" || len(got.Table.Rows) != 1 {
		t.Fatalf("release data was clobbered: %#v", got)
	}
	if !got.SupportFresh() || len(got.Support) != 1 || got.Support[0].EOLDate != "2026-07-17" {
		t.Fatalf("unexpected support data: %#v", got.Support)
	}
}

func TestSaveReleasePreservesSupportData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")

	if err := SaveSupport(path, sampleSupport()); err != nil {
		t.Fatalf("SaveSupport: %v", err)
	}
	p := samplePayload(time.Now())
	if err := SaveRelease(path, p.Table, p.Resolved); err != nil {
		t.Fatalf("SaveRelease: %v", err)
	}

	got, err := LoadStale(path)
	if err != nil {
		t.Fatalf("LoadStale: %v", err)
	}
	if len(got.Support) != 1 || got.Support[0].Minor != "1.21" {
		t.Fatalf("support data was clobbered: %#v", got.Support)
	}
	if got.Resolved["1.21"] != "1.21.2" {
		t.Fatalf("unexpected resolved: %v", got.Resolved)
	}
}

// A cache file written before support data existed must still decode, with
// support simply reported as stale so it gets refetched once.
func TestLoadStaleLegacyPayload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	legacy := `{"fetched_at":"2026-01-01T00:00:00Z","table":{"Headers":[],"Components":[],"Rows":[]},"resolved":{"1.21":"1.21.2"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadStale(path)
	if err != nil {
		t.Fatalf("LoadStale: %v", err)
	}
	if got.Resolved["1.21"] != "1.21.2" {
		t.Fatalf("unexpected resolved: %v", got.Resolved)
	}
	if got.SupportFresh() || len(got.Support) != 0 {
		t.Fatalf("expected no cached support data, got %#v", got.Support)
	}
}

func TestLoadStaleReturnsExpiredPayload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, samplePayload(time.Now().Add(-8*24*time.Hour))); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadStale(path)
	if err != nil {
		t.Fatalf("LoadStale: %v", err)
	}
	if got.Resolved["1.21"] != "1.21.2" {
		t.Fatalf("expected stale payload to be returned, got %#v", got)
	}
}

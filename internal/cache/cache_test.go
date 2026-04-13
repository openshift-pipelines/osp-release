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

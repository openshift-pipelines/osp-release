package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/openshift-pipelines/osp-release/internal/release"
)

const TTL = 7 * 24 * time.Hour

// Payload is the combined Confluence+Jira+life cycle data stored on disk.
type Payload struct {
	FetchedAt        time.Time               `json:"fetched_at"`
	Table            release.Table           `json:"table"`
	Resolved         map[string]string       `json:"resolved"` // minor -> jira version string
	Support          []release.SupportRecord `json:"support,omitempty"`
	SupportFetchedAt time.Time               `json:"support_fetched_at,omitempty"`
}

// SupportFresh reports whether the cached life cycle data is within the TTL.
func (p Payload) SupportFresh() bool {
	return !p.SupportFetchedAt.IsZero() && time.Since(p.SupportFetchedAt) <= TTL
}

// Load reads the cache at path. Returns (payload, true, nil) on a fresh hit,
// (zero, false, nil) if the file is missing or older than TTL, and
// (zero, false, err) if the file exists but cannot be decoded.
func Load(path string) (Payload, bool, error) {
	p, err := LoadStale(path)
	if err != nil {
		return Payload{}, false, err
	}
	if p.FetchedAt.IsZero() || time.Since(p.FetchedAt) > TTL {
		return Payload{}, false, nil
	}
	return p, true, nil
}

// LoadStale reads the cache at path without applying the TTL. A missing file
// yields a zero payload and no error, so callers can use it to merge new data
// into an existing cache without dropping unrelated fields.
func LoadStale(path string) (Payload, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Payload{}, nil
	}
	if err != nil {
		return Payload{}, err
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, err
	}
	return p, nil
}

// Save atomically writes p to path by writing a temp file then renaming it.
func Save(path string, p Payload) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SaveRelease stores Confluence/Jira data while preserving cached life cycle
// data already on disk.
func SaveRelease(path string, table release.Table, resolved map[string]string) error {
	existing, err := LoadStale(path)
	if err != nil {
		existing = Payload{}
	}
	existing.FetchedAt = time.Now()
	existing.Table = table
	existing.Resolved = resolved
	return Save(path, existing)
}

// SaveSupport stores life cycle data while preserving cached Confluence/Jira
// data already on disk.
func SaveSupport(path string, support []release.SupportRecord) error {
	existing, err := LoadStale(path)
	if err != nil {
		existing = Payload{}
	}
	existing.SupportFetchedAt = time.Now()
	existing.Support = support
	return Save(path, existing)
}

// DefaultPath returns $XDG_CACHE_HOME/osp-release/release-data.json (or
// the OS equivalent via os.UserCacheDir).
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "osp-release", "release-data.json"), nil
}

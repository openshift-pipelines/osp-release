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

// Payload is the combined Confluence+Jira data stored on disk.
type Payload struct {
	FetchedAt time.Time         `json:"fetched_at"`
	Table     release.Table     `json:"table"`
	Resolved  map[string]string `json:"resolved"` // minor -> jira version string
}

// Load reads the cache at path. Returns (payload, true, nil) on a fresh hit,
// (zero, false, nil) if the file is missing or older than TTL, and
// (zero, false, err) if the file exists but cannot be decoded.
func Load(path string) (Payload, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Payload{}, false, nil
	}
	if err != nil {
		return Payload{}, false, err
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, false, err
	}
	if time.Since(p.FetchedAt) > TTL {
		return Payload{}, false, nil
	}
	return p, true, nil
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

// DefaultPath returns $XDG_CACHE_HOME/osp-release/release-data.json (or
// the OS equivalent via os.UserCacheDir).
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "osp-release", "release-data.json"), nil
}

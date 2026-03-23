package release

import (
	"slices"
	"strconv"
	"strings"
)

const (
	Site       = "https://redhat.atlassian.net"
	PageID     = 267160127
	ProjectKey = "SRVKP"
)

type Table struct {
	Headers    []string
	Components []string
	Rows       []ReleaseRow
}

type ReleaseRow struct {
	Minor   string
	Values  map[string]string
	Filled  int
	RawHTML string
}

func (r ReleaseRow) IsReleased() bool {
	return r.Filled > 5 && !strings.Contains(r.RawHTML, "TODO")
}

type ReleaseRecord struct {
	Minor      string
	Version    string
	Released   bool
	Components map[string]string
}

func (r ReleaseRecord) ToMap() map[string]any {
	fields := map[string]any{
		"minor":    r.Minor,
		"version":  r.Version,
		"released": r.Released,
	}
	for _, name := range UpstreamComponentNames() {
		fields[name] = r.Components[name]
	}
	return fields
}

type UpstreamRecord struct {
	Component string
	Repo      string
	Version   string
}

func (r UpstreamRecord) ToMap() map[string]any {
	return map[string]any{
		"component": r.Component,
		"repo":      r.Repo,
		"version":   r.Version,
	}
}

func NormalizeHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "-", " ")
	parts := strings.Fields(header)
	return strings.Join(parts, "_")
}

func ParseMinor(version string) []int {
	parts := strings.Split(version, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return []int{}
		}
		out = append(out, value)
	}
	return out
}

func CompareVersions(left, right string) int {
	lv := ParseMinor(left)
	rv := ParseMinor(right)
	return slices.Compare(lv, rv)
}

func SortRows(rows []ReleaseRow) {
	slices.SortFunc(rows, func(left, right ReleaseRow) int {
		return CompareVersions(left.Minor, right.Minor)
	})
}

func DefaultReleaseFields() []string {
	return []string{"minor", "version", "released"}
}

func DefaultUpstreamFields() []string {
	return []string{"component", "version", "repo"}
}

func BuildRecords(table Table, resolved map[string]string) []ReleaseRecord {
	records := make([]ReleaseRecord, 0, len(table.Rows))
	for _, row := range table.Rows {
		version := resolved[row.Minor]
		if version == "" {
			version = row.Minor
		}
		components := make(map[string]string, len(table.Components))
		for _, name := range table.Components {
			components[name] = row.Values[name]
		}
		records = append(records, ReleaseRecord{
			Minor:      row.Minor,
			Version:    version,
			Released:   row.IsReleased(),
			Components: components,
		})
	}
	slices.Reverse(records)
	return records
}

func AllowedReleaseFields() []string {
	fields := []string{"minor", "version", "released"}
	fields = append(fields, UpstreamComponentNames()...)
	return fields
}

func AllowedUpstreamFields() []string {
	return []string{"component", "repo", "version"}
}

func LatestReleased(records []ReleaseRecord) (ReleaseRecord, bool) {
	for _, record := range records {
		if record.Released {
			return record, true
		}
	}
	return ReleaseRecord{}, false
}

func LatestUnreleased(records []ReleaseRecord) (ReleaseRecord, bool) {
	for _, record := range records {
		if !record.Released {
			return record, true
		}
	}
	return ReleaseRecord{}, false
}

func FindByMinor(records []ReleaseRecord, minor string) (ReleaseRecord, bool) {
	for _, record := range records {
		if record.Minor == minor {
			return record, true
		}
	}
	return ReleaseRecord{}, false
}

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

	// LifecycleURL is the public Red Hat product life cycle endpoint. It needs
	// no credentials.
	LifecycleURL     = "https://access.redhat.com/product-life-cycles/api/v1/products"
	LifecycleProduct = "Red Hat OpenShift Pipelines"
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
	Support    SupportRecord
}

func (r ReleaseRecord) ToMap() map[string]any {
	fields := map[string]any{
		"minor":            r.Minor,
		"version":          r.Version,
		"released":         r.Released,
		"support_status":   r.Support.SupportStatus,
		"ga_date":          r.Support.GADate,
		"full_support_end": r.Support.FullSupportEnd,
		"eol_date":         r.Support.EOLDate,
	}
	for _, name := range UpstreamComponentNames() {
		fields[name] = r.Components[name]
	}
	return fields
}

// SupportRecord holds the Red Hat product life cycle data for one OSP minor.
type SupportRecord struct {
	Minor          string
	SupportStatus  string
	GADate         string
	FullSupportEnd string
	EOLDate        string
}

func (r SupportRecord) ToMap() map[string]any {
	return map[string]any{
		"minor":            r.Minor,
		"support_status":   r.SupportStatus,
		"ga_date":          r.GADate,
		"full_support_end": r.FullSupportEnd,
		"eol_date":         r.EOLDate,
	}
}

// StatusEndOfLife is the life cycle status Red Hat reports for versions that
// have left maintenance support.
const StatusEndOfLife = "End of life"

// IsSupported reports whether a version is still under full or maintenance
// support. Records with no known status are treated as supported so missing
// life cycle data never hides a release.
func (r SupportRecord) IsSupported() bool {
	return !strings.EqualFold(strings.TrimSpace(r.SupportStatus), StatusEndOfLife)
}

// FilterSupported drops end-of-life entries.
func FilterSupported(records []SupportRecord) []SupportRecord {
	filtered := make([]SupportRecord, 0, len(records))
	for _, record := range records {
		if record.IsSupported() {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// SortSupportRecords orders records from newest to oldest minor.
func SortSupportRecords(records []SupportRecord) {
	slices.SortFunc(records, func(left, right SupportRecord) int {
		return CompareVersions(right.Minor, left.Minor)
	})
}

// AttachSupport fills the Support field of each release record from the
// matching life cycle entry. Records without a match keep an empty support
// record, and life cycle entries without a matching release are ignored.
func AttachSupport(records []ReleaseRecord, support []SupportRecord) {
	if len(support) == 0 {
		return
	}
	byMinor := make(map[string]SupportRecord, len(support))
	for _, entry := range support {
		byMinor[entry.Minor] = entry
	}
	for idx := range records {
		if entry, ok := byMinor[records[idx].Minor]; ok {
			records[idx].Support = entry
		}
	}
}

// FindSupportByMinor returns the life cycle entry for a minor version.
func FindSupportByMinor(records []SupportRecord, minor string) (SupportRecord, bool) {
	for _, record := range records {
		if record.Minor == minor {
			return record, true
		}
	}
	return SupportRecord{}, false
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
	return []string{"minor", "version", "released", "support_status"}
}

// SupportFields are the life cycle fields carried by release records.
func SupportFields() []string {
	return []string{"support_status", "ga_date", "full_support_end", "eol_date"}
}

func DefaultSupportFields() []string {
	return []string{"minor", "support_status", "ga_date", "full_support_end", "eol_date"}
}

func AllowedSupportFields() []string {
	return DefaultSupportFields()
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
	fields = append(fields, SupportFields()...)
	fields = append(fields, UpstreamComponentNames()...)
	return fields
}

// ReleaseFieldsWithComponents is the default field set used when component
// versions are shown. It keeps the wide table readable by including only
// support_status from the life cycle fields.
func ReleaseFieldsWithComponents() []string {
	fields := []string{"minor", "version", "released", "support_status"}
	return append(fields, UpstreamComponentNames()...)
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

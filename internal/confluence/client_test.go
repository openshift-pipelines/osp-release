package confluence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTable(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "testdata", "confluence_page.html")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	table, err := ParseTable(string(content))
	if err != nil {
		t.Fatalf("ParseTable() error = %v", err)
	}

	if got, want := len(table.Rows), 3; got != want {
		t.Fatalf("len(table.Rows) = %d, want %d", got, want)
	}

	if table.Rows[0].Minor != "1.20" || table.Rows[1].Minor != "1.21" || table.Rows[2].Minor != "1.22" {
		t.Fatalf("unexpected row order: %+v", table.Rows)
	}

	if table.Components[0] != "pac" {
		t.Fatalf("unexpected first component: %q", table.Components[0])
	}

	if !table.Rows[0].IsReleased() {
		t.Fatalf("1.20 should be released")
	}
	if table.Rows[2].IsReleased() {
		t.Fatalf("1.22 should be unreleased")
	}
	if table.Rows[0].Values["pac"] != "0.37.x" {
		t.Fatalf("unexpected PAC version: %q", table.Rows[0].Values["pac"])
	}
}

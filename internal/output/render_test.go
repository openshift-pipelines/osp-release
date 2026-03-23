package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSelectFormat(t *testing.T) {
	t.Parallel()

	if got, _ := SelectFormat("", false, true); got != FormatTable {
		t.Fatalf("tty default = %q", got)
	}
	if got, _ := SelectFormat("", false, false); got != FormatJSON {
		t.Fatalf("non-tty default = %q", got)
	}
	if got, _ := SelectFormat("text", false, true); got != FormatText {
		t.Fatalf("explicit format = %q", got)
	}
}

func TestRendererJSONFieldSelection(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	renderer := Renderer{Stdout: &out}
	data := Dataset{
		Rows: []map[string]any{
			{"minor": "1.21", "version": "1.21.2", "pac": "0.39.x"},
		},
		Singular:      true,
		AllowedFields: []string{"minor", "version", "pac"},
		DefaultFields: []string{"minor", "version"},
	}

	if err := renderer.Render(data, FormatJSON, []string{"version", "pac"}, false, false); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["version"] != "1.21.2" || payload["pac"] != "0.39.x" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestRendererTable(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	renderer := Renderer{Stdout: &out, IsTTY: true}
	data := Dataset{
		Rows: []map[string]any{
			{"component": "pac", "version": "v0.40.1", "repo": "tektoncd/pipelines-as-code"},
		},
		Singular:      false,
		AllowedFields: []string{"component", "version", "repo"},
		DefaultFields: []string{"component", "version"},
	}

	if err := renderer.Render(data, FormatTable, nil, false, false); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "╭") || !strings.Contains(rendered, "Component") || !strings.Contains(rendered, "pac") {
		t.Fatalf("unexpected table output: %q", rendered)
	}
}

func TestRendererTableNoHeaders(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	renderer := Renderer{Stdout: &out, IsTTY: true}
	data := Dataset{
		Rows: []map[string]any{
			{"component": "pac", "value": "0.39.x"},
		},
		Singular:      false,
		AllowedFields: []string{"component", "value"},
		DefaultFields: []string{"component", "value"},
	}

	if err := renderer.Render(data, FormatTable, nil, false, true); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	rendered := strings.ToLower(out.String())
	if strings.Contains(rendered, "component") || !strings.Contains(rendered, "0.39.x") {
		t.Fatalf("unexpected no-header table output: %q", rendered)
	}
}

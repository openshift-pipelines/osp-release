package output

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	prettytable "github.com/jedib0t/go-pretty/v6/table"
	prettytext "github.com/jedib0t/go-pretty/v6/text"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/release"
)

type Format string

const (
	FormatTable Format = "table"
	FormatText  Format = "text"
	FormatJSON  Format = "json"
)

type Dataset struct {
	Rows          []map[string]any
	Singular      bool
	AllowedFields []string
	DefaultFields []string
}

type Renderer struct {
	Stdout io.Writer
	IsTTY  bool
}

func SelectFormat(requested string, wantJSON bool, isTTY bool) (Format, error) {
	if wantJSON {
		return FormatJSON, nil
	}
	if requested == "" {
		if isTTY {
			return FormatTable, nil
		}
		return FormatJSON, nil
	}

	switch Format(strings.ToLower(strings.TrimSpace(requested))) {
	case FormatTable, FormatText, FormatJSON:
		return Format(strings.ToLower(strings.TrimSpace(requested))), nil
	default:
		return "", app.New(
			app.KindInvalidOutputFormat,
			fmt.Sprintf("invalid output format %q", requested),
			app.ExitUsage,
			map[string]any{"format": requested},
			nil,
		)
	}
}

func (r Renderer) Render(data Dataset, format Format, fields []string, quiet bool, noHeaders bool) error {
	if quiet {
		if len(fields) != 1 {
			return app.New(app.KindInvalidQuietUsage, "--quiet requires exactly one --field", app.ExitUsage, nil, nil)
		}
		return r.renderQuiet(data.Rows, fields[0])
	}

	switch format {
	case FormatJSON:
		return r.renderJSON(data, fields)
	case FormatText:
		return r.renderText(data, fields)
	case FormatTable:
		return r.renderTable(data, fields, noHeaders)
	default:
		return app.New(app.KindInvalidOutputFormat, "unsupported output format", app.ExitUsage, nil, nil)
	}
}

func (r Renderer) renderJSON(data Dataset, fields []string) error {
	rows, err := projectRows(data, fields, false)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(r.Stdout)
	if data.Singular {
		if len(rows) == 0 {
			return encoder.Encode(map[string]any{})
		}
		return encoder.Encode(rows[0])
	}
	return encoder.Encode(rows)
}

func (r Renderer) renderText(data Dataset, fields []string) error {
	columns, err := selectedFields(data, fields, false)
	if err != nil {
		return err
	}
	rows, err := projectRows(data, columns, true)
	if err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, stringify(row[column]))
		}
		if _, err := fmt.Fprintln(r.Stdout, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) renderTable(data Dataset, fields []string, noHeaders bool) error {
	columns, err := selectedFields(data, fields, false)
	if err != nil {
		return err
	}
	rows, err := projectRows(data, columns, true)
	if err != nil {
		return err
	}

	writer := prettytable.NewWriter()
	writer.SetOutputMirror(r.Stdout)
	writer.SetStyle(prettyTableStyle(r.IsTTY))
	writer.SetColumnConfigs(columnConfigs(columns))
	if !noHeaders {
		header := make(prettytable.Row, 0, len(columns))
		for _, column := range columns {
			header = append(header, humanizeHeader(column))
		}
		writer.AppendHeader(header)
	}
	for _, row := range rows {
		record := make(prettytable.Row, 0, len(columns))
		for _, column := range columns {
			record = append(record, stringify(row[column]))
		}
		writer.AppendRow(record)
	}
	writer.Render()
	return nil
}

func (r Renderer) renderQuiet(rows []map[string]any, field string) error {
	for _, row := range rows {
		value, ok := row[field]
		if !ok {
			return app.New(app.KindInvalidField, fmt.Sprintf("unknown field %q", field), app.ExitUsage, map[string]any{"field": field}, nil)
		}
		if _, err := fmt.Fprintln(r.Stdout, stringify(value)); err != nil {
			return err
		}
	}
	return nil
}

func projectRows(data Dataset, fields []string, requireSelection bool) ([]map[string]any, error) {
	columns, err := selectedFields(data, fields, requireSelection)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(data.Rows))
	for _, row := range data.Rows {
		projected := make(map[string]any, len(columns))
		for _, column := range columns {
			projected[column] = row[column]
		}
		rows = append(rows, projected)
	}
	return rows, nil
}

func selectedFields(data Dataset, requested []string, requireSelection bool) ([]string, error) {
	if len(requested) == 0 {
		if requireSelection {
			return data.DefaultFields, nil
		}
		return data.DefaultFields, nil
	}
	for _, field := range requested {
		if !slices.Contains(data.AllowedFields, field) {
			return nil, app.New(app.KindInvalidField, fmt.Sprintf("unknown field %q", field), app.ExitUsage, map[string]any{"field": field}, nil)
		}
	}
	return requested, nil
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func humanizeHeader(value string) string {
	if strings.Contains(value, ".") {
		if display := release.DisplayName(value); display != value {
			return display
		}
	}
	if display := release.DisplayName(value); display != value {
		return display
	}
	replacer := strings.NewReplacer("_", " ", ".", " ")
	return prettytext.FormatTitle.Apply(replacer.Replace(value))
}

func prettyTableStyle(isTTY bool) prettytable.Style {
	style := prettytable.StyleRounded
	style.Format.Header = prettytext.FormatDefault
	style.Options.SeparateRows = false
	style.Options.SeparateColumns = true
	style.Options.SeparateHeader = true
	style.Options.DrawBorder = true
	style.Box.PaddingLeft = " "
	style.Box.PaddingRight = " "
	if isTTY {
		style.Color.Border = prettytext.Colors{prettytext.FgHiBlack}
		style.Color.Separator = prettytext.Colors{prettytext.FgHiBlack}
		style.Color.Header = prettytext.Colors{prettytext.Bold, prettytext.FgHiCyan}
		style.Color.Row = prettytext.Colors{prettytext.FgWhite}
		style.Color.RowAlternate = prettytext.Colors{prettytext.FgHiWhite}
	}
	return style
}

func columnConfigs(columns []string) []prettytable.ColumnConfig {
	configs := make([]prettytable.ColumnConfig, 0, len(columns))
	for _, column := range columns {
		config := prettytable.ColumnConfig{
			Name:        humanizeHeader(column),
			AlignHeader: prettytext.AlignLeft,
			Align:       prettytext.AlignLeft,
		}
		if column == "released" {
			config.Align = prettytext.AlignCenter
			config.AlignHeader = prettytext.AlignCenter
		}
		configs = append(configs, config)
	}
	return configs
}

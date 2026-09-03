package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/cache"
	"github.com/openshift-pipelines/osp-release/internal/output"
	"github.com/openshift-pipelines/osp-release/internal/release"
	"github.com/openshift-pipelines/osp-release/internal/version"
)

// describeSchema is the machine-readable contract of the CLI. It exists so
// automation never has to scrape --help or guess valid field names.
type describeSchema struct {
	Binary     string            `json:"binary"`
	Version    string            `json:"version"`
	Commands   []describeCommand `json:"commands"`
	ExitCodes  []app.ExitCode    `json:"exit_codes"`
	ErrorKinds []app.KindInfo    `json:"error_kinds"`
	Env        []describeEnv     `json:"env"`
	Cache      describeCache     `json:"cache"`
}

type describeCommand struct {
	Path   string          `json:"path"`
	Short  string          `json:"short"`
	Args   []describeArg   `json:"args"`
	Flags  []describeFlag  `json:"flags"`
	Fields *describeFields `json:"fields,omitempty"`
	Auth   []string        `json:"auth,omitempty"`
	Cached bool            `json:"cached"`
}

type describeArg struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	Values   []string `json:"values,omitempty"`
}

type describeFlag struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
	Inherited   bool   `json:"inherited"`
}

type describeFields struct {
	Allowed []string `json:"allowed"`
	Default []string `json:"default"`
}

type describeEnv struct {
	Name            string   `json:"name"`
	RequiredFor     []string `json:"required_for"`
	SupportsPassRef bool     `json:"supports_pass_ref"`
	Description     string   `json:"description"`
}

type describeCache struct {
	TTLDays int    `json:"ttl_days"`
	Path    string `json:"path"`
	Bypass  string `json:"bypass"`
}

// commandMeta holds the parts of the contract cobra does not know about.
type commandMeta struct {
	allowed []string
	deflt   []string
	auth    []string
	cached  bool
}

var jiraAuth = []string{"OSP_JIRA_EMAIL", "OSP_JIRA_TOKEN"}

// commandMetadata is keyed by command path. Field lists reference the same
// helpers the renderer validates against, so they cannot drift apart.
// TestDescribeCoversEveryCommand fails if a new command has no entry here.
func commandMetadata() map[string]commandMeta {
	releaseMeta := commandMeta{
		allowed: release.AllowedReleaseFields(),
		deflt:   release.DefaultReleaseFields(),
		auth:    jiraAuth,
		cached:  true,
	}
	upstreamMeta := commandMeta{
		allowed: release.AllowedUpstreamFields(),
		deflt:   release.DefaultUpstreamFields(),
	}
	supportMeta := commandMeta{
		allowed: release.AllowedSupportFields(),
		deflt:   release.DefaultSupportFields(),
		cached:  true,
	}

	return map[string]commandMeta{
		"release latest":     releaseMeta,
		"release unreleased": releaseMeta,
		"release list":       releaseMeta,
		"release all":        releaseMeta,
		"release show":       releaseMeta,
		"component list": {
			allowed: []string{"name"},
			deflt:   []string{"name"},
			auth:    jiraAuth,
			cached:  true,
		},
		"component show": {
			allowed: []string{"component", "display_name", "repo", "latest_upstream"},
			deflt:   []string{"component", "display_name", "repo", "latest_upstream"},
			auth:    jiraAuth,
			cached:  true,
		},
		"upstream list":     upstreamMeta,
		"upstream show":     upstreamMeta,
		"support list":      supportMeta,
		"support show":      supportMeta,
		"version":           {allowed: version.Fields(), deflt: version.Fields()},
		"describe":          {},
		"describe commands": {},
		"describe fields":   {},
	}
}

func newDescribeCommand(a application, opts *rootOptions) *cobra.Command {
	describeCmd := &cobra.Command{
		Use:   "describe [commands|fields] [command path]",
		Short: "Print the machine-readable command and field schema",
		Long: "Print the command surface, selectable fields, exit codes, and error kinds as JSON.\n" +
			"Use this instead of parsing --help. Output is JSON by default, even on a terminal.",
		Example: "  osp-release describe\n" +
			"  osp-release describe commands\n" +
			"  osp-release describe commands release show\n" +
			"  osp-release describe fields\n" +
			"  osp-release describe fields release show",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.renderDescribe(opts, buildSchema(cmd.Root()), describeViewFull)
		},
	}

	commandsCmd := &cobra.Command{
		Use:               "commands [command path]",
		Short:             "Describe every command, or one command path",
		Example:           "  osp-release describe commands\n  osp-release describe commands release show",
		ValidArgsFunction: describePathCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := filterSchema(buildSchema(cmd.Root()), args)
			if err != nil {
				return err
			}
			return a.renderDescribe(opts, schema, describeViewCommands)
		},
	}

	fieldsCmd := &cobra.Command{
		Use:               "fields [command path]",
		Short:             "Describe the selectable --field names",
		Example:           "  osp-release describe fields\n  osp-release describe fields release show",
		ValidArgsFunction: describePathCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := filterSchema(buildSchema(cmd.Root()), args)
			if err != nil {
				return err
			}
			return a.renderDescribe(opts, schema, describeViewFields)
		},
	}

	describeCmd.AddCommand(commandsCmd, fieldsCmd)
	return describeCmd
}

type describeView int

const (
	describeViewFull describeView = iota
	describeViewCommands
	describeViewFields
)

func buildSchema(root *cobra.Command) describeSchema {
	metadata := commandMetadata()
	schema := describeSchema{
		Binary:     root.Name(),
		Version:    version.Get().Version,
		Commands:   collectCommands(root, metadata),
		ExitCodes:  app.ExitCodes(),
		ErrorKinds: app.Kinds(),
		Env: []describeEnv{
			{
				Name:            "OSP_JIRA_EMAIL",
				RequiredFor:     []string{"release", "component"},
				SupportsPassRef: true,
				Description:     "Jira account email",
			},
			{
				Name:            "OSP_JIRA_TOKEN",
				RequiredFor:     []string{"release", "component"},
				SupportsPassRef: true,
				Description:     "Jira API token",
			},
		},
		Cache: describeCache{
			TTLDays: int(cache.TTL.Hours() / 24),
			Path:    cachePathForDescribe(),
			Bypass:  "--refresh",
		},
	}
	return schema
}

func cachePathForDescribe() string {
	path, err := cache.DefaultPath()
	if err != nil {
		return ""
	}
	return filepath.Dir(path)
}

// collectCommands walks the live cobra tree so the schema cannot drift from
// the commands that actually exist.
func collectCommands(root *cobra.Command, metadata map[string]commandMeta) []describeCommand {
	commands := make([]describeCommand, 0, len(metadata))
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			if child.Runnable() {
				commands = append(commands, describeOne(root, child, metadata))
			}
			walk(child)
		}
	}
	walk(root)
	slices.SortFunc(commands, func(left, right describeCommand) int {
		return strings.Compare(left.Path, right.Path)
	})
	return commands
}

func describeOne(root *cobra.Command, cmd *cobra.Command, metadata map[string]commandMeta) describeCommand {
	path := commandPath(root, cmd)
	described := describeCommand{
		Path:  path,
		Short: cmd.Short,
		Args:  describeArgs(cmd),
		Flags: describeFlags(cmd),
	}
	if meta, ok := metadata[path]; ok {
		if len(meta.allowed) > 0 {
			described.Fields = &describeFields{Allowed: meta.allowed, Default: meta.deflt}
		}
		described.Auth = meta.auth
		described.Cached = meta.cached
	}
	return described
}

func commandPath(root *cobra.Command, cmd *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name()))
}

// describeArgs reads the positional arguments out of the Use line, for example
// "show <minor>" or "commands [command path]".
func describeArgs(cmd *cobra.Command) []describeArg {
	fields := strings.Fields(cmd.Use)
	if len(fields) < 2 {
		return []describeArg{}
	}
	args := make([]describeArg, 0, len(fields)-1)
	for _, field := range fields[1:] {
		switch {
		case strings.HasPrefix(field, "<") && strings.HasSuffix(field, ">"):
			args = append(args, describeArg{
				Name:     strings.Trim(field, "<>"),
				Required: true,
				Values:   argValues(cmd),
			})
		case strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]"):
			args = append(args, describeArg{Name: strings.Trim(field, "[]"), Required: false})
		}
	}
	return args
}

func argValues(cmd *cobra.Command) []string {
	if strings.Contains(cmd.Use, "<component>") || strings.Contains(cmd.Use, "<name>") {
		return release.UpstreamComponentNames()
	}
	return nil
}

func describeFlags(cmd *cobra.Command) []describeFlag {
	flags := make([]describeFlag, 0)
	add := func(flag *pflag.Flag, inherited bool) {
		if flag.Hidden {
			return
		}
		flags = append(flags, describeFlag{
			Name:        flag.Name,
			Type:        flag.Value.Type(),
			Default:     flag.DefValue,
			Description: flag.Usage,
			Inherited:   inherited,
		})
	}
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) { add(flag, false) })
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) { add(flag, true) })
	slices.SortFunc(flags, func(left, right describeFlag) int {
		return strings.Compare(left.Name, right.Name)
	})
	return flags
}

func filterSchema(schema describeSchema, args []string) (describeSchema, error) {
	if len(args) == 0 {
		return schema, nil
	}
	wanted := strings.Join(args, " ")
	for _, command := range schema.Commands {
		if command.Path == wanted {
			schema.Commands = []describeCommand{command}
			return schema, nil
		}
	}
	return describeSchema{}, app.New(
		app.KindUnknownCommand,
		fmt.Sprintf("unknown command %q", wanted),
		app.ExitNotFound,
		map[string]any{"command": wanted},
		nil,
	)
}

func describePathCompletion(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	paths := make([]string, 0)
	prefix := strings.Join(args, " ")
	for _, command := range buildSchema(cmd.Root()).Commands {
		if prefix == "" {
			paths = append(paths, strings.Fields(command.Path)[0])
			continue
		}
		if rest, ok := strings.CutPrefix(command.Path, prefix+" "); ok {
			paths = append(paths, strings.Fields(rest)[0])
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths), cobra.ShellCompDirectiveNoFileComp
}

// renderDescribe defaults to JSON even on a terminal: this command exists for
// automation. Humans can still ask for --output table.
func (a application) renderDescribe(opts *rootOptions, schema describeSchema, view describeView) error {
	format, err := output.SelectFormat(opts.output, opts.json, false)
	if err != nil {
		return err
	}

	if format == output.FormatJSON {
		return a.encodeDescribeJSON(schema, view)
	}

	renderer := output.Renderer{Stdout: a.streams.out, IsTTY: a.streams.isTTY}
	if view == describeViewFields {
		rows := make([]map[string]any, 0, len(schema.Commands))
		for _, command := range schema.Commands {
			if command.Fields == nil {
				continue
			}
			rows = append(rows, map[string]any{
				"path":           command.Path,
				"allowed_fields": strings.Join(command.Fields.Allowed, ","),
				"default_fields": strings.Join(command.Fields.Default, ","),
			})
		}
		return renderer.Render(output.Dataset{
			Rows:          rows,
			AllowedFields: []string{"path", "allowed_fields", "default_fields"},
			DefaultFields: []string{"path", "allowed_fields", "default_fields"},
		}, format, opts.fields, opts.quiet, opts.noHeaders)
	}

	rows := make([]map[string]any, 0, len(schema.Commands))
	for _, command := range schema.Commands {
		rows = append(rows, map[string]any{
			"path":   command.Path,
			"short":  command.Short,
			"auth":   strings.Join(command.Auth, ","),
			"cached": command.Cached,
		})
	}
	return renderer.Render(output.Dataset{
		Rows:          rows,
		AllowedFields: []string{"path", "short", "auth", "cached"},
		DefaultFields: []string{"path", "short", "auth", "cached"},
	}, format, opts.fields, opts.quiet, opts.noHeaders)
}

func (a application) encodeDescribeJSON(schema describeSchema, view describeView) error {
	encoder := json.NewEncoder(a.streams.out)
	encoder.SetIndent("", "  ")
	switch view {
	case describeViewCommands:
		if len(schema.Commands) == 1 {
			return encoder.Encode(schema.Commands[0])
		}
		return encoder.Encode(map[string]any{"commands": schema.Commands})
	case describeViewFields:
		type fieldEntry struct {
			Path    string   `json:"path"`
			Allowed []string `json:"allowed"`
			Default []string `json:"default"`
		}
		entries := make([]fieldEntry, 0, len(schema.Commands))
		for _, command := range schema.Commands {
			if command.Fields == nil {
				continue
			}
			entries = append(entries, fieldEntry{
				Path:    command.Path,
				Allowed: command.Fields.Allowed,
				Default: command.Fields.Default,
			})
		}
		if len(entries) == 1 {
			return encoder.Encode(entries[0])
		}
		return encoder.Encode(map[string]any{"fields": entries})
	case describeViewFull:
		return encoder.Encode(schema)
	default:
		return encoder.Encode(schema)
	}
}

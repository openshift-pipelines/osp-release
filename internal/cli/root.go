package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/openshift-pipelines/osp-release/internal/app"
	"github.com/openshift-pipelines/osp-release/internal/cache"
	"github.com/openshift-pipelines/osp-release/internal/confluence"
	"github.com/openshift-pipelines/osp-release/internal/credentials"
	"github.com/openshift-pipelines/osp-release/internal/jira"
	"github.com/openshift-pipelines/osp-release/internal/output"
	"github.com/openshift-pipelines/osp-release/internal/release"
	"github.com/openshift-pipelines/osp-release/internal/upstream"
)

type streams struct {
	out   io.Writer
	err   io.Writer
	isTTY bool
}

type rootOptions struct {
	output       string
	json         bool
	quiet        bool
	noHeaders    bool
	components   bool
	noComponents bool
	unreleased   bool
	fields       []string
	jiraEmail    string
	jiraToken    string
	debug        bool
	refresh      bool
}

type application struct {
	streams        streams
	httpClient     *http.Client
	creds          credentials.Resolver
	upstreamClient *upstream.Client
	siteURL        string
	pageID         int
	projectKey     string
	cacheDir       string // if non-empty, overrides cache.DefaultPath() directory
}

func Run(ctx context.Context, stdout, stderr io.Writer, isTTY bool) int {
	upstreamClient, err := upstream.NewClient()
	if err != nil {
		renderError(stderr, stdout, wantsJSONOutput(os.Args[1:], isTTY), err)
		if appErr, ok := app.Extract(err); ok {
			return appErr.Exit
		}
		return app.ExitGeneral
	}

	application := application{
		streams: streams{
			out:   stdout,
			err:   stderr,
			isTTY: isTTY,
		},
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		creds:          credentials.NewResolver(),
		upstreamClient: upstreamClient,
		siteURL:        release.Site,
		pageID:         release.PageID,
		projectKey:     release.ProjectKey,
	}
	command := newRootCommand(ctx, application)
	command.SetErr(stderr)
	command.SetOut(stdout)

	if err := command.ExecuteContext(ctx); err != nil {
		renderError(stderr, stdout, wantsJSONOutput(os.Args[1:], isTTY), err)
		if appErr, ok := app.Extract(err); ok {
			return appErr.Exit
		}
		return app.ExitGeneral
	}
	return app.ExitSuccess
}

func newRootCommand(ctx context.Context, application application) *cobra.Command {
	opts := &rootOptions{components: true}

	root := &cobra.Command{
		Use:   "osp-release",
		Short: "Query OpenShift Pipelines release data",
		Example: "  osp-release release latest\n" +
			"  osp-release release show 1.21\n" +
			"  osp-release release list --json\n" +
			"  osp-release component list\n" +
			"  osp-release upstream list",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&opts.output, "output", "", "Output format: table, text, json")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "Shorthand for --output json")
	root.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "Print only the selected field value")
	root.PersistentFlags().BoolVar(&opts.noHeaders, "no-headers", false, "Hide table headers")
	root.PersistentFlags().StringSliceVar(&opts.fields, "field", nil, "Field to output; repeat to select multiple fields")
	_ = root.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Query release versions from Confluence and Jira",
		Example: "  osp-release release latest\n" +
			"  osp-release release unreleased --output text\n" +
			"  osp-release release show 1.21\n" +
			"  osp-release release list --json\n" +
			"  osp-release release all --unreleased",
	}
	releaseCmd.PersistentFlags().StringVar(&opts.jiraEmail, "jira-email", "", "Jira email or pass::entry")
	releaseCmd.PersistentFlags().StringVar(&opts.jiraToken, "jira-token", "", "Jira token or pass::entry")
	releaseCmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "Print debug messages to stderr")
	releaseCmd.PersistentFlags().BoolVar(&opts.components, "components", true, "Include component versions in release output")
	releaseCmd.PersistentFlags().BoolVar(&opts.noComponents, "no-components", false, "Hide component versions in release output")
	releaseCmd.PersistentFlags().BoolVar(&opts.unreleased, "unreleased", false, "Include unreleased versions in release listings")
	releaseCmd.PersistentFlags().BoolVar(&opts.refresh, "refresh", false, "Bypass cache and fetch live Jira/Confluence data")

	componentCmd := &cobra.Command{
		Use:   "component",
		Short: "Inspect release components",
		Example: "  osp-release component list\n" +
			"  osp-release component list --json",
	}
	componentCmd.PersistentFlags().StringVar(&opts.jiraEmail, "jira-email", "", "Jira email or pass::entry")
	componentCmd.PersistentFlags().StringVar(&opts.jiraToken, "jira-token", "", "Jira token or pass::entry")
	componentCmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "Print debug messages to stderr")
	componentCmd.PersistentFlags().BoolVar(&opts.refresh, "refresh", false, "Bypass cache and fetch live Jira/Confluence data")

	upstreamCmd := &cobra.Command{
		Use:   "upstream",
		Short: "Query latest upstream release tags",
		Example: "  osp-release upstream list\n" +
			"  osp-release upstream show pac --json",
	}

	releaseCmd.AddCommand(
		&cobra.Command{
			Use:   "latest",
			Short: "Show the latest released OSP version",
			Example: "  osp-release release latest\n" +
				"  osp-release release latest --field version\n" +
				"  osp-release release latest --no-components",
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.loadReleaseRecords(cmd.Context(), opts)
				if err != nil {
					return err
				}
				record, ok := release.LatestReleased(records)
				if !ok {
					return app.New("release_not_found", "no released versions found", app.ExitNotFound, nil, nil)
				}
				return application.renderRecord(cmd.Context(), opts, []map[string]any{record.ToMap()}, true, release.AllowedReleaseFields(), release.DefaultReleaseFields())
			},
		},
		&cobra.Command{
			Use:   "unreleased",
			Short: "Show the latest unreleased OSP version",
			Example: "  osp-release release unreleased\n" +
				"  osp-release release unreleased --json",
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.loadReleaseRecords(cmd.Context(), opts)
				if err != nil {
					return err
				}
				record, ok := release.LatestUnreleased(records)
				if !ok {
					return app.New("release_not_found", "no unreleased versions found", app.ExitNotFound, nil, nil)
				}
				return application.renderRecord(cmd.Context(), opts, []map[string]any{record.ToMap()}, true, release.AllowedReleaseFields(), release.DefaultReleaseFields())
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all OSP release versions",
			Example: "  osp-release release list\n" +
				"  osp-release release list --unreleased\n" +
				"  osp-release release list --no-components\n" +
				"  osp-release release list --field minor --field version --output text",
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.loadReleaseRecords(cmd.Context(), opts)
				if err != nil {
					return err
				}
				records = filterListedRecords(records, opts.unreleased)
				rows := make([]map[string]any, 0, len(records))
				for _, record := range records {
					rows = append(rows, record.ToMap())
				}
				return application.renderRecord(cmd.Context(), opts, rows, false, release.AllowedReleaseFields(), release.DefaultReleaseFields())
			},
		},
		&cobra.Command{
			Use:   "all",
			Short: "List all OSP release versions",
			Example: "  osp-release release all\n" +
				"  osp-release release all --unreleased\n" +
				"  osp-release release all --no-components\n" +
				"  osp-release release all --field minor --field version --output text",
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.loadReleaseRecords(cmd.Context(), opts)
				if err != nil {
					return err
				}
				records = filterListedRecords(records, opts.unreleased)
				rows := make([]map[string]any, 0, len(records))
				for _, record := range records {
					rows = append(rows, record.ToMap())
				}
				return application.renderRecord(cmd.Context(), opts, rows, false, release.AllowedReleaseFields(), release.DefaultReleaseFields())
			},
		},
		&cobra.Command{
			Use:   "show <minor>",
			Short: "Show one OSP release version",
			Args:  cobra.ExactArgs(1),
			Example: "  osp-release release show 1.21\n" +
				"  osp-release release show 1.21 --no-components",
			ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.loadReleaseRecords(cmd.Context(), opts)
				if err != nil {
					return err
				}
				record, ok := release.FindByMinor(records, args[0])
				if !ok {
					return app.New("release_not_found", fmt.Sprintf("release %q not found", args[0]), app.ExitNotFound, map[string]any{"minor": args[0]}, nil)
				}
				return application.renderRecord(cmd.Context(), opts, []map[string]any{record.ToMap()}, true, release.AllowedReleaseFields(), release.DefaultReleaseFields())
			},
		},
	)

	componentCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List release component names",
			Example: "  osp-release component list\n" +
				"  osp-release component list --quiet --field name",
			RunE: func(cmd *cobra.Command, args []string) error {
				table, _, err := application.loadCachedData(cmd.Context(), opts)
				if err != nil {
					return err
				}
				rows := make([]map[string]any, 0, len(table.Components))
				for _, component := range table.Components {
					rows = append(rows, map[string]any{"name": component})
				}
				return application.renderRecord(cmd.Context(), opts, rows, false, []string{"name"}, []string{"name"})
			},
		},
		&cobra.Command{
			Use:   "show <name>",
			Short: "Show detailed information about one component",
			Args:  cobra.ExactArgs(1),
			Example: "  osp-release component show pac\n" +
				"  osp-release component show pac --json\n" +
				"  osp-release component show pac --field latest_upstream --quiet",
			ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				if len(args) != 0 {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				return release.UpstreamComponentNames(), cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				return application.renderComponentDetail(cmd.Context(), opts, args[0])
			},
		},
	)

	upstreamCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List latest upstream releases for all known components",
			Example: "  osp-release upstream list\n" +
				"  osp-release upstream list --field component --field version --output text",
			RunE: func(cmd *cobra.Command, args []string) error {
				records, err := application.upstreamClient.LatestReleases(cmd.Context())
				if err != nil {
					return err
				}
				rows := make([]map[string]any, 0, len(records))
				for _, record := range records {
					rows = append(rows, record.ToMap())
				}
				return application.renderRecord(cmd.Context(), opts, rows, false, release.AllowedUpstreamFields(), release.DefaultUpstreamFields())
			},
		},
		&cobra.Command{
			Use:   "show <component>",
			Short: "Show the latest upstream release for one component",
			Args:  cobra.ExactArgs(1),
			Example: "  osp-release upstream show pac\n" +
				"  osp-release upstream show pac --field version --quiet",
			ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				if len(args) != 0 {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				return release.UpstreamComponentNames(), cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				record, err := application.upstreamClient.LatestRelease(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return application.renderRecord(cmd.Context(), opts, []map[string]any{record.ToMap()}, true, release.AllowedUpstreamFields(), release.DefaultUpstreamFields())
			},
		},
	)

	root.AddCommand(releaseCmd, componentCmd, upstreamCmd)
	_ = ctx
	return root
}

func (a application) loadReleaseRecords(ctx context.Context, opts *rootOptions) ([]release.ReleaseRecord, error) {
	table, resolved, err := a.loadCachedData(ctx, opts)
	if err != nil {
		return nil, err
	}
	records := release.BuildRecords(table, resolved)
	if opts.noComponents || !opts.components {
		return records, nil
	}
	for idx := range records {
		for component, version := range records[idx].Components {
			records[idx].Components[component] = a.upstreamClient.ResolveComponentVersion(ctx, component, version)
		}
	}
	return records, nil
}

// loadCachedData returns the Confluence table and Jira-resolved version map.
// It reads from the on-disk cache when it is fresh and opts.refresh is false;
// otherwise it fetches live data and writes the result to the cache.
func (a application) loadCachedData(ctx context.Context, opts *rootOptions) (release.Table, map[string]string, error) {
	cachePath, pathErr := a.resolveCachePath()
	if pathErr == nil && !opts.refresh {
		if p, fresh, err := cache.Load(cachePath); err == nil && fresh {
			return p.Table, p.Resolved, nil
		}
		// corrupt file or expired — fall through to live fetch
	}

	table, err := a.loadTable(ctx, opts)
	if err != nil {
		return release.Table{}, nil, err
	}
	resolver, err := a.loadResolver(ctx, opts)
	if err != nil {
		return release.Table{}, nil, err
	}
	resolved := make(map[string]string, len(table.Rows))
	for _, row := range table.Rows {
		resolved[row.Minor] = resolver.Resolve(row.Minor)
	}

	// Only write cache on full success.
	if pathErr == nil {
		_ = cache.Save(cachePath, cache.Payload{
			FetchedAt: time.Now(),
			Table:     table,
			Resolved:  resolved,
		})
	}
	return table, resolved, nil
}

func (a application) resolveCachePath() (string, error) {
	if a.cacheDir != "" {
		return filepath.Join(a.cacheDir, "release-data.json"), nil
	}
	return cache.DefaultPath()
}

func (a application) loadTable(ctx context.Context, opts *rootOptions) (release.Table, error) {
	creds, err := a.loadCredentials(ctx, opts)
	if err != nil {
		return release.Table{}, err
	}
	client := confluence.Client{
		HTTPClient: a.httpClient,
		BaseURL:    a.siteURL,
		PageID:     a.pageID,
	}
	return client.FetchTable(ctx, creds, a.debugWriter(opts))
}

func (a application) loadResolver(ctx context.Context, opts *rootOptions) (jira.Resolver, error) {
	creds, err := a.loadCredentials(ctx, opts)
	if err != nil {
		return jira.Resolver{}, err
	}
	client := jira.Client{
		HTTPClient: a.httpClient,
		BaseURL:    a.siteURL,
		ProjectKey: a.projectKey,
	}
	return client.FetchResolver(ctx, creds, a.debugWriter(opts))
}

func (a application) loadCredentials(ctx context.Context, opts *rootOptions) (credentials.Credentials, error) {
	return a.creds.Resolve(
		ctx,
		os.Getenv("OSP_JIRA_EMAIL"),
		os.Getenv("OSP_JIRA_TOKEN"),
		opts.jiraEmail,
		opts.jiraToken,
	)
}

func (a application) renderRecord(ctx context.Context, opts *rootOptions, rows []map[string]any, singular bool, allowedFields, defaultFields []string) error {
	_ = ctx
	format, err := output.SelectFormat(opts.output, opts.json, a.streams.isTTY)
	if err != nil {
		return err
	}
	isReleaseRecord := slices.Equal(allowedFields, release.AllowedReleaseFields())
	if opts.components && !opts.noComponents && isReleaseRecord && len(opts.fields) == 0 {
		if singular && format == output.FormatTable && len(rows) == 1 {
			renderer := output.Renderer{Stdout: a.streams.out, IsTTY: a.streams.isTTY}
			if err := renderer.Render(output.Dataset{
				Rows:          rows,
				Singular:      singular,
				AllowedFields: allowedFields,
				DefaultFields: defaultFields,
			}, format, opts.fields, opts.quiet, opts.noHeaders); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(a.streams.out); err != nil {
				return err
			}
			return renderer.Render(output.Dataset{
				Rows:          componentRows(rows[0]),
				Singular:      false,
				AllowedFields: []string{"component", "value"},
				DefaultFields: []string{"component", "value"},
			}, format, nil, false, opts.noHeaders)
		}
		defaultFields = append([]string(nil), allowedFields...)
	}
	renderer := output.Renderer{Stdout: a.streams.out, IsTTY: a.streams.isTTY}
	return renderer.Render(output.Dataset{
		Rows:          rows,
		Singular:      singular,
		AllowedFields: allowedFields,
		DefaultFields: defaultFields,
	}, format, opts.fields, opts.quiet, opts.noHeaders)
}

func componentRows(row map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(release.UpstreamComponentNames()))
	for _, component := range release.UpstreamComponentNames() {
		rows = append(rows, map[string]any{
			"component": component,
			"value":     row[component],
		})
	}
	return rows
}

func (a application) renderComponentDetail(ctx context.Context, opts *rootOptions, component string) error {
	component = strings.ToLower(strings.TrimSpace(component))
	repo, ok := release.UpstreamRepo(component)
	if !ok {
		return app.New("unknown_component", fmt.Sprintf("unknown component %q", component), app.ExitNotFound, map[string]any{"component": component}, nil)
	}

	records, err := a.loadReleaseRecords(ctx, opts)
	if err != nil {
		return err
	}

	upstreamRecord, err := a.upstreamClient.LatestRelease(ctx, component)
	if err != nil {
		return err
	}

	summary := map[string]any{
		"component":       component,
		"display_name":    release.DisplayName(component),
		"repo":            repo,
		"latest_upstream": upstreamRecord.Version,
	}

	history := make([]map[string]any, 0, len(records))
	for _, record := range records {
		history = append(history, map[string]any{
			"minor":             record.Minor,
			"version":           record.Version,
			"released":          record.Released,
			"component_version": record.Components[component],
		})
	}

	format, err := output.SelectFormat(opts.output, opts.json, a.streams.isTTY)
	if err != nil {
		return err
	}

	if len(opts.fields) > 0 {
		renderer := output.Renderer{Stdout: a.streams.out, IsTTY: a.streams.isTTY}
		return renderer.Render(output.Dataset{
			Rows:          []map[string]any{summary},
			Singular:      true,
			AllowedFields: []string{"component", "display_name", "repo", "latest_upstream"},
			DefaultFields: []string{"component", "display_name", "repo", "latest_upstream"},
		}, format, opts.fields, opts.quiet, opts.noHeaders)
	}

	if format == output.FormatJSON {
		return json.NewEncoder(a.streams.out).Encode(map[string]any{
			"component":       component,
			"display_name":    release.DisplayName(component),
			"repo":            repo,
			"latest_upstream": upstreamRecord.Version,
			"releases":        history,
		})
	}

	renderer := output.Renderer{Stdout: a.streams.out, IsTTY: a.streams.isTTY}
	if err := renderer.Render(output.Dataset{
		Rows:          []map[string]any{summary},
		Singular:      true,
		AllowedFields: []string{"component", "display_name", "repo", "latest_upstream"},
		DefaultFields: []string{"component", "display_name", "repo", "latest_upstream"},
	}, format, nil, false, opts.noHeaders); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.streams.out); err != nil {
		return err
	}
	return renderer.Render(output.Dataset{
		Rows:          history,
		Singular:      false,
		AllowedFields: []string{"minor", "version", "released", "component_version"},
		DefaultFields: []string{"minor", "version", "released", "component_version"},
	}, format, nil, false, opts.noHeaders)
}

func filterListedRecords(records []release.ReleaseRecord, includeUnreleased bool) []release.ReleaseRecord {
	if includeUnreleased {
		return records
	}
	filtered := make([]release.ReleaseRecord, 0, len(records))
	for _, record := range records {
		if record.Released {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (a application) debugWriter(opts *rootOptions) io.Writer {
	if !opts.debug {
		return nil
	}
	return a.streams.err
}

func renderError(stderr, stdout io.Writer, useJSON bool, err error) {
	if err == nil {
		return
	}

	appErr, ok := app.Extract(err)
	if ok {
		_, _ = fmt.Fprintln(stderr, "Error:", appErr.Message)
		if useJSON {
			payload := map[string]any{
				"error":   appErr.Kind,
				"message": appErr.Message,
			}
			for key, value := range appErr.Fields {
				payload[key] = value
			}
			_ = json.NewEncoder(stdout).Encode(payload)
		}
		return
	}

	_, _ = fmt.Fprintln(stderr, "Error:", err)
	if useJSON {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"error":   "general_failure",
			"message": err.Error(),
		})
	}
}

func IsTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func wantsJSONOutput(args []string, isTTY bool) bool {
	for idx := 0; idx < len(args); idx++ {
		switch args[idx] {
		case "--json":
			return true
		case "--output=json":
			return true
		case "--output":
			if idx+1 < len(args) {
				return args[idx+1] == "json"
			}
		}
	}
	return !isTTY
}

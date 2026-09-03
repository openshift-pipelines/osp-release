package app

// Error kinds carried by Error.Kind. They are part of the machine-readable
// contract: agents and scripts match on these instead of on message text.
const (
	KindConfluenceAuthFailed    = "confluence_auth_failed"
	KindConfluenceDecodeFailed  = "confluence_decode_failed"
	KindConfluenceRequestFailed = "confluence_request_failed"
	KindGeneralFailure          = "general_failure"
	KindGitHubAuthFailed        = "github_auth_failed"
	KindGitHubClientFailed      = "github_client_failed"
	KindGitHubRequestFailed     = "github_request_failed"
	KindHeadersNotFound         = "headers_not_found"
	KindHTMLParseFailed         = "html_parse_failed"
	KindInvalidField            = "invalid_field"
	KindInvalidOutputFormat     = "invalid_output_format"
	KindInvalidPassReference    = "invalid_pass_reference"
	KindInvalidQuietUsage       = "invalid_quiet_usage"
	KindJiraAuthFailed          = "jira_auth_failed"
	KindJiraDecodeFailed        = "jira_decode_failed"
	KindJiraRequestFailed       = "jira_request_failed"
	KindLifecycleDecodeFailed   = "lifecycle_decode_failed"
	KindLifecycleProductMissing = "lifecycle_product_not_found"
	KindLifecycleRequestFailed  = "lifecycle_request_failed"
	KindMissingCredentials      = "missing_credentials"
	KindPassLookupFailed        = "pass_lookup_failed"
	KindReleaseNotFound         = "release_not_found"
	KindRequestBuildFailed      = "request_build_failed"
	KindRowsNotFound            = "rows_not_found"
	KindSupportNotFound         = "support_not_found"
	KindTableNotFound           = "table_not_found"
	KindUnknownCommand          = "unknown_command"
	KindUnknownComponent        = "unknown_component"
)

// ExitCode describes one process exit status.
type ExitCode struct {
	Name    string `json:"name"`
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// ExitCodes returns every exit status the CLI can produce.
func ExitCodes() []ExitCode {
	return []ExitCode{
		{Name: "success", Code: ExitSuccess, Meaning: "command completed"},
		{Name: "general", Code: ExitGeneral, Meaning: "unexpected failure"},
		{Name: "usage", Code: ExitUsage, Meaning: "invalid flag, field, or argument"},
		{Name: "not_found", Code: ExitNotFound, Meaning: "the requested release, component, or command does not exist"},
		{Name: "auth", Code: ExitAuth, Meaning: "missing or rejected credentials"},
		{Name: "dependency", Code: ExitDependency, Meaning: "a required external tool failed"},
		{Name: "upstream", Code: ExitUpstream, Meaning: "an upstream service returned an error"},
	}
}

// KindInfo describes one error kind and the exit status it maps to.
type KindInfo struct {
	Name    string `json:"name"`
	Exit    int    `json:"exit"`
	Meaning string `json:"meaning"`
}

// Kinds returns every error kind the CLI can report.
func Kinds() []KindInfo {
	return []KindInfo{
		{KindConfluenceAuthFailed, ExitAuth, "Confluence rejected the Jira credentials"},
		{KindConfluenceDecodeFailed, ExitGeneral, "the Confluence response could not be decoded"},
		{KindConfluenceRequestFailed, ExitUpstream, "the Confluence request failed"},
		{KindGeneralFailure, ExitGeneral, "an untyped failure"},
		{KindGitHubAuthFailed, ExitAuth, "GitHub rejected the request"},
		{KindGitHubClientFailed, ExitDependency, "the GitHub client could not be initialized"},
		{KindGitHubRequestFailed, ExitUpstream, "the GitHub request failed"},
		{KindHeadersNotFound, ExitGeneral, "the Confluence release table has no headers"},
		{KindHTMLParseFailed, ExitGeneral, "the Confluence page could not be parsed"},
		{KindInvalidField, ExitUsage, "an unknown --field name was requested"},
		{KindInvalidOutputFormat, ExitUsage, "an unknown --output format was requested"},
		{KindInvalidPassReference, ExitUsage, "a pass:: reference is malformed"},
		{KindInvalidQuietUsage, ExitUsage, "--quiet needs exactly one --field"},
		{KindJiraAuthFailed, ExitAuth, "Jira rejected the credentials"},
		{KindJiraDecodeFailed, ExitGeneral, "the Jira response could not be decoded"},
		{KindJiraRequestFailed, ExitUpstream, "the Jira request failed"},
		{KindLifecycleDecodeFailed, ExitGeneral, "the life cycle response could not be decoded"},
		{KindLifecycleProductMissing, ExitUpstream, "the life cycle API has no data for the product"},
		{KindLifecycleRequestFailed, ExitUpstream, "the life cycle request failed"},
		{KindMissingCredentials, ExitAuth, "Jira credentials are not set"},
		{KindPassLookupFailed, ExitDependency, "the pass lookup failed"},
		{KindReleaseNotFound, ExitNotFound, "the requested release does not exist"},
		{KindRequestBuildFailed, ExitGeneral, "the HTTP request could not be built"},
		{KindRowsNotFound, ExitGeneral, "the Confluence release table has no rows"},
		{KindSupportNotFound, ExitNotFound, "no life cycle data for the requested version"},
		{KindTableNotFound, ExitGeneral, "the Confluence page has no release table"},
		{KindUnknownCommand, ExitNotFound, "the described command path does not exist"},
		{KindUnknownComponent, ExitNotFound, "the requested component is not in the catalog"},
	}
}

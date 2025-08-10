// flags.go
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// cliFlags holds all parsed CLI flags and arguments.
type cliFlags struct {
	showHelp           bool
	extended           bool
	quiet              bool
	machine            bool
	includeDescRegex   string
	excludeDescRegex   string
	dryRun             bool
	summary            bool
	toolNameFormat     string
	diffFile           string
	tagFlags           multiFlag
	docFile            string
	docFormat          string
	postHookCmd        string
	noConfirmDangerous bool
	args               []string
	mounts             mountFlags // slice of mountFlag
	functionListFile   string     // Path to file listing functions to include (for filter command)
	logFile            string     // Path to file for logging MCP requests and responses
	noLogTruncation    bool       // Disable truncation in human-readable MCP logs
}

type mountFlag struct {
	BasePath string
	SpecPath string
}

type mountFlags []mountFlag

func (m *mountFlags) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *mountFlags) Set(val string) error {
	// Expect format: /base:path/to/spec.yaml
	sep := strings.Index(val, ":")
	if sep < 1 || sep == len(val)-1 {
		return fmt.Errorf("invalid --mount value: %q (expected /base:path/to/spec.yaml)", val)
	}
	*m = append(*m, mountFlag{
		BasePath: val[:sep],
		SpecPath: val[sep+1:],
	})
	return nil
}

// parseFlags parses all CLI flags and returns a cliFlags struct.
func parseFlags() *cliFlags {
	var flags cliFlags
	flag.BoolVar(&flags.showHelp, "h", false, "Show help")
	flag.BoolVar(&flags.showHelp, "help", false, "Show help")
	flag.BoolVar(&flags.extended, "extended", false, "Enable extended (human-friendly) output")
	// Default to minimal output
	flags.quiet = true
	flags.machine = true
	flag.StringVar(&flags.includeDescRegex, "include-desc-regex", "", "Only include APIs whose description matches this regex (overrides INCLUDE_DESC_REGEX env)")
	flag.StringVar(&flags.excludeDescRegex, "exclude-desc-regex", "", "Exclude APIs whose description matches this regex (overrides EXCLUDE_DESC_REGEX env)")
	flag.BoolVar(&flags.dryRun, "dry-run", false, "Print the generated MCP tool schemas and exit (do not start the server)")
	flag.Var(&flags.tagFlags, "tag", "Only include tools with the given OpenAPI tag (repeatable)")
	flag.StringVar(&flags.toolNameFormat, "tool-name-format", "", "Format tool names: lower, upper, snake, camel")
	flag.BoolVar(&flags.summary, "summary", false, "Print a summary of the generated tools (count, tags, etc)")
	flag.StringVar(&flags.diffFile, "diff", "", "Compare the generated output to a previous run (file path)")
	flag.StringVar(&flags.docFile, "doc", "", "Write Markdown/HTML documentation for all tools to this file (implies no server)")
	flag.StringVar(&flags.docFormat, "doc-format", "markdown", "Documentation format: markdown (default) or html")
	flag.StringVar(&flags.postHookCmd, "post-hook-cmd", "", "Command to post-process the generated tool schema JSON (used in --dry-run or --doc mode)")
	flag.BoolVar(&flags.noConfirmDangerous, "no-confirm-dangerous", false, "Disable confirmation prompt for dangerous (PUT/POST/DELETE) actions in tool descriptions")
	flag.Var(&flags.mounts, "mount", "Mount an OpenAPI spec at a base path: /base:path/to/spec.yaml (repeatable, can be used multiple times)")
	flag.StringVar(&flags.functionListFile, "function-list-file", "", "File with list of function (operationId) names to include (one per line, for filter command)")
	flag.StringVar(&flags.logFile, "log-file", "", "File path to log all MCP requests and responses for debugging")
	flag.BoolVar(&flags.noLogTruncation, "no-log-truncation", false, "Disable truncation of long values in human-readable MCP logs")
	flag.Parse()
	flags.args = flag.Args()
	if flags.extended {
		flags.quiet = false
		flags.machine = false
	}
	return &flags
}

// printHelp prints the CLI help message.
func printHelp() {
	fmt.Print(`openapi-mcp: Expose OpenAPI APIs as MCP tools

Usage:
  openapi-mcp [flags] filter <openapi-spec-path>
  openapi-mcp [flags] validate <openapi-spec-path>
  openapi-mcp [flags] lint <openapi-spec-path>
  openapi-mcp [flags] <openapi-spec-path>

Commands:
  filter <openapi-spec-path>    Output a filtered list of operations as JSON, applying --tag, --include-desc-regex, --exclude-desc-regex, and --function-list-file (no server)
  validate <openapi-spec-path>  Validate the OpenAPI spec and report actionable errors (with --http: starts validation API server)
  lint <openapi-spec-path>      Perform detailed OpenAPI linting with comprehensive suggestions (with --http: starts linting API server)

Examples:

  Validation & Linting:
    openapi-mcp validate api.yaml                 # Check for critical issues
    openapi-mcp lint api.yaml                     # Comprehensive linting

  Filtering & Documentation:
    openapi-mcp filter --tag=admin api.yaml              # Only admin operations
    openapi-mcp filter --dry-run api.yaml                # Preview generated tools
    openapi-mcp filter --doc=tools.md api.yaml           # Generate documentation
    openapi-mcp filter --tag=admin api.yaml              # Output only admin-tagged operations as JSON
    openapi-mcp filter --include-desc-regex=foo api.yaml # Output operations whose description matches 'foo'
    openapi-mcp filter --function-list-file=funcs.txt api.yaml # Output only operations listed in funcs.txt

  Advanced Configuration:
    openapi-mcp --include-desc-regex="user.*" api.yaml      # Filter by description
    openapi-mcp --no-confirm-dangerous api.yaml             # Skip confirmations

Flags:
  --extended           Enable extended (human-friendly) output (default: minimal/agent)
  --include-desc-regex Only include APIs whose description matches this regex
  --exclude-desc-regex Exclude APIs whose description matches this regex
  --dry-run            Print the generated MCP tool schemas as JSON and exit
  --doc                Write Markdown/HTML documentation for all tools to this file
  --doc-format         Documentation format: markdown (default) or html
  --post-hook-cmd      Command to post-process the generated tool schema JSON
  --no-confirm-dangerous Disable confirmation for dangerous actions
  --summary            Print a summary for CI
  --tag                Only include tools with the given tag
  --diff               Compare generated tools with a reference file
  --mount /base:path/to/spec.yaml  Mount an OpenAPI spec at a base path (repeatable, can be used multiple times)
  --function-list-file   File with list of function (operationId) names to include (one per line, for filter command)
  --log-file           File path to log all MCP requests and responses for debugging
  --no-log-truncation  Disable truncation of long values in human-readable MCP logs
  --help, -h           Show help

By default, output is minimal and agent-friendly. Use --extended for banners, help, and human-readable output.
`)
	os.Exit(0)
}

// multiFlag is a custom flag type for collecting repeated string values.
type multiFlag []string

// String returns the string representation of the multiFlag.
func (m *multiFlag) String() string {
	return fmt.Sprintf("%v", *m)
}

// Set appends a value to the multiFlag.
func (m *multiFlag) Set(val string) error {
	*m = append(*m, val)
	return nil
}

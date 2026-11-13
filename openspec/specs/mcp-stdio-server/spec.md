# mcp-stdio-server Specification

## Purpose

Defines the stdio-based MCP server lifecycle, configuration handling, logging behavior, and error propagation contract for the `forage-mcp` binary.

## Requirements

### Requirement: Server communicates over stdio transport
The system SHALL run as a long-lived process that reads MCP requests from standard input and writes MCP responses to standard output, conforming to the Model Context Protocol stdio transport specification.

#### Scenario: MCP client spawns the server
- **WHEN** an MCP-compatible client launches the `forage-mcp` binary and sends an MCP `initialize` request over stdin
- **THEN** the system responds over stdout with a valid MCP `initialize` result identifying the server name, version, and capabilities

#### Scenario: Server receives a tool call over stdio
- **WHEN** an MCP client sends a `tools/call` request for `forage_search` or `forage_extract` over stdin
- **THEN** the system processes the request and writes the corresponding MCP result back to stdout

#### Scenario: Server receives an unknown tool name
- **WHEN** an MCP client sends a `tools/call` request for a tool name other than `forage_search` or `forage_extract`
- **THEN** the system returns an MCP protocol-level error indicating the tool is not found, without crashing the process

### Requirement: Configuration is loaded from environment and CLI flags
The system SHALL load configuration from environment variables first, then allow CLI flags to override those values. The Forage base URL MUST be resolvable from either `FORAGE_URL` or `-u/--forage-url`; if neither is set, the server MUST fail at startup with a clear error.

#### Scenario: FORAGE_URL is set via environment variable
- **WHEN** the process starts with `FORAGE_URL=http://localhost:3672` and no `-u` flag
- **THEN** the system uses `http://localhost:3672` as the Forage base URL

#### Scenario: -u flag overrides FORAGE_URL
- **WHEN** the process starts with `FORAGE_URL=http://env-host:3672` and `-u http://flag-host:3672`
- **THEN** the system uses `http://flag-host:3672` as the Forage base URL

#### Scenario: No Forage URL is configured
- **WHEN** the process starts without `FORAGE_URL` set and without a `-u` flag
- **THEN** the system exits with a non-zero status and prints an error message indicating that `FORAGE_URL` or `--forage-url` is required

#### Scenario: API key is provided via environment variable
- **WHEN** the process starts with `FORAGE_API_KEY=secret-token` set
- **THEN** all outbound HTTP requests to Forage include header `Authorization: Bearer secret-token`

#### Scenario: -k flag overrides FORAGE_API_KEY
- **WHEN** the process starts with `FORAGE_API_KEY=env-key` and `-k flag-key`
- **THEN** all outbound HTTP requests to Forage include header `Authorization: Bearer flag-key`

#### Scenario: No API key is configured
- **WHEN** the process starts without `FORAGE_API_KEY` set and without a `-k` flag
- **THEN** outbound HTTP requests to Forage do not include an `Authorization` header

### Requirement: Global timeout is configurable
The system SHALL use a global default timeout of 120 seconds for outbound Forage API calls, overridable via the `-t/--timeout` CLI flag.

#### Scenario: Default timeout applies when no flag is given
- **WHEN** the process starts without a `-t` flag
- **THEN** outbound HTTP requests to Forage use a 120-second timeout

#### Scenario: -t flag overrides the default timeout
- **WHEN** the process starts with `-t 60`
- **THEN** outbound HTTP requests to Forage use a 60-second timeout, unless a per-request timeout parameter overrides it

### Requirement: Logging is plaintext on stderr with configurable verbosity
The system SHALL log all diagnostic output as plaintext (not JSON) to standard error using Go's `log/slog` text handler. Log level SHALL be controllable via `-v/--verbose` (debug), default (info), and `-q/--quiet` (warn), or via the `LOG_LEVEL` environment variable.

#### Scenario: Default verbosity logs incoming requests
- **WHEN** the process starts without `-v`, `-q`, or `LOG_LEVEL`
- **THEN** the system logs at info level, including a line for each incoming MCP tool call with the tool name and parameters

#### Scenario: -v enables debug logging
- **WHEN** the process starts with `-v`
- **THEN** the system logs at debug level, including detailed request/response information

#### Scenario: -q suppresses info-level logs
- **WHEN** the process starts with `-q`
- **THEN** the system logs only warnings and errors to stderr

#### Scenario: LOG_LEVEL environment variable sets verbosity
- **WHEN** the process starts with `LOG_LEVEL=debug` set and no `-v` or `-q` flag
- **THEN** the system logs at debug level

#### Scenario: Logs never go to stdout
- **WHEN** the system produces any log output
- **THEN** all log lines are written to stderr, keeping stdout reserved exclusively for MCP protocol messages

### Requirement: Tool errors are returned as MCP error results
The system SHALL return tool-level failures (validation errors, HTTP errors from Forage, timeouts) as Go errors from the tool handler, which the MCP SDK SHALL surface as an MCP result with `isError: true` and a human-readable error message in the content.

#### Scenario: Validation error is returned to the client
- **WHEN** a tool call fails client-side validation (e.g., empty query)
- **THEN** the system returns an MCP result with `isError: true` and a content string that identifies the invalid parameter and its constraint, without calling Forage

#### Scenario: Forage HTTP error is forwarded to the client
- **WHEN** the Forage backend responds with a non-2xx HTTP status code
- **THEN** the system returns an MCP result with `isError: true` and a content string that includes the HTTP status code and the Forage response body

#### Scenario: Timeout error is returned to the client
- **WHEN** an outbound request to Forage exceeds its effective timeout
- **THEN** the system returns an MCP result with `isError: true` and a content string indicating the request timed out, including the timeout duration in seconds

#### Scenario: Internal errors are logged but not exposed
- **WHEN** an unexpected internal error occurs during tool processing
- **THEN** the system logs full details at debug level to stderr and returns an MCP result with `isError: true` containing a generic error message, without exposing stack traces or internal state to the client

# Proposal: Forage MCP Server

## Project Overview

**forage-mcp** is an MCP (Model Context Protocol) server written in Go that bridges AI agents to the [Forage](https://github.com/aldemaroc/forage) web search and content extraction service. It exposes two tools — `forage_search` and `forage_extract` — via stdio transport, allowing AI assistants (Claude Desktop, Agent Zero, or any MCP-compatible client) to perform web searches and extract clean content from arbitrary URLs.

Forage is a self-hosted service that provides:
- **Web Search** via multiple SERP providers (Google, Bing, Yahoo, DuckDuckGo, Brave) or SearXNG
- **Web Extract** for clean content extraction using hybrid static + browser methods

This server acts as a thin, fast wrapper around Forage's Native API — no proxy logic, no caching of its own, just structured I/O between the MCP client and the Forage backend.

## Goals

- Provide two well-defined MCP tools: `forage_search` and `forage_extract`
- Use only Forage's **Native Hermes-compatible** API (`/search`, `/extract`)
- Communicate via **stdio transport** (suitable for Claude Desktop, Agent Zero, and CLI integration)
- Written in **Go 1.25**, compiled to a single standalone binary
- Built on the **official Go MCP SDK** (`github.com/modelcontextprotocol/go-sdk`)
- Accept configuration via environment variables and CLI arguments using `github.com/spf13/pflag`
- Include comprehensive plaintext logging to stderr with configurable verbosity
- Validate inputs robustly (URLs, enum values) before forwarding to Forage

## Non-Goals

- Browser control or headless rendering within the Go binary (handled by Forage server)
- Authentication token management (auth is configured on the Forage server)
- Multi-instance process management (no systemd/Supervisor integration)

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go 1.25 | Fast compilation, small binary, strong stdlib |
| Transport | stdio | Standard for MCP servers; works with Claude Desktop, Agent Zero, CLI tools |
| API Mode | Native Hermes-compatible only | Simpler contract; Firecrawl compatibility is out of scope |
| Binary Name | `forage-mcp` | Clear purpose, consistent naming convention |
| Config | Env vars + CLI args via `pflag` | Flexible for containerized and local deployments |
| Logging | stderr, plaintext, configurable verbosity via envvar or CLI arg | Human-readable; simple to integrate |
| Argument Parser | `github.com/spf13/pflag` (v1.0+) | Matches dark-penguin/llm-tools; well-tested, POSIX-compatible |
| MCP SDK | Official `github.com/modelcontextprotocol/go-sdk` | Idiomatic stdio server + tool registration; handles result/error marshaling |
| Module Path | `github.com/dark-penguin/forage-mcp` | Matches the (future) public repo; valid even while private/local |

## Forage API Mapping

How tool parameters map onto Forage's Native API (some via headers, not just body fields):

- **`no_cache`** → when `true`, the request is sent with a `Cache-Control: no-cache` header to bypass Forage's in-memory cache; when `false`/omitted, the header is not sent.
- **`cache_status`** (result field) → populated from the **`X-Forage-Cache` response header** (`hit | miss | bypass | disabled`); it is not present in Forage's JSON body.
- **Timeouts** → the per-request `timeout` parameter on `forage_extract` **overrides** the global `-t/--timeout`; the global value is the fallback. `forage_search` has no per-request timeout, so it always uses the global.

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `FORAGE_URL` | Yes | Base URL of the Forage instance (e.g., `http://localhost:3672`) |
| `FORAGE_API_KEY` | No | Optional Bearer token for authenticated Forage instances |

## CLI Arguments

| Flag | Description | Default |
|---|---|---|
| `-u`,`--forage-url` | Override `FORAGE_URL` | (envvar) |
| `-k`,`--forage-api-key` | Override `FORAGE_API_KEY` | (envvar) |
| `-t`,`--timeout` | Default timeout for API calls in seconds | `120` |
| `-v`,`--verbose` | Set log level to DEBUG (log incoming requests) | off |
| `-q`,`--quiet` | Set log level to WARN (do not log outgoing Forage requests) | off |

## MCP Tools

### Tool 1: `forage_search`

Search the web using Forage's configured search provider.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | Yes | Search query (1–500 chars) |
| `limit` | integer | No | Number of results (1–50). Default: 5 |
| `language` | string | No | Language code (e.g., `pt-BR`, `en`). Default: Forage config default |
| `engines` | array[string] | No | Search engines to use. Values: `google`, `bing`, `yahoo`, `duckduckgo`, `brave`. Default: Forage configured providers |
| `no_cache` | boolean | No | Skip Forage's in-memory cache (sent as `Cache-Control: no-cache`). Default: false |

**Response Schema (MCP tool result):**

```jsonc
{
  "success": true,
  "data": {
    "web": [
      {
        "title": "Page Title",
        "url": "https://example.com/page",
        "description": "Page description text",
        "position": 1
      }
    ]
  },
  "cache_status": "hit" // from X-Forage-Cache header: "hit", "miss", "bypass", "disabled"
}
```

**Validation:**
- `query`: non-empty string, max 500 chars
- `limit`: integer in range 1–50 (validated server-side if not provided)
- `language`: pass-through (Forage validates against supported locales)
- `engines`: all values must be valid engine names (`google`, `bing`, `yahoo`, `duckduckgo`, `brave`); the `ddg` alias for DuckDuckGo is also accepted

### Tool 2: `forage_extract`

Extract clean content from web pages using Forage's hybrid extraction pipeline.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `urls` | array[string] | Yes | URLs to extract (1–20). Must start with `http://` or `https://` |
| `formats` | array[string] | No | Output formats. Values: `markdown` (default), `html`, `raw_html` |
| `only_main_content` | boolean | No | Extract only main content, stripping nav/footer. Default: true |
| `force_render` | boolean | No | Force browser-based rendering. Default: false |
| `wait_for` | string | No | CSS selector to wait for before extraction (browser mode). Default: none |
| `timeout` | integer | No | Per-request timeout in seconds (1–120); overrides the global `-t/--timeout`. Default: 30 |
| `engine` | string | No | Extraction engine: `trafilatura` or `readability`. Default: Forage default |
| `no_cache` | boolean | No | Skip Forage's in-memory cache (sent as `Cache-Control: no-cache`). Default: false |

**Response Schema (MCP tool result):**

```jsonc
{
  "success": true,
  "data": [
    {
      "url": "https://example.com",
      "title": "Page Title",
      "content": "Clean markdown content...",
      "raw_content": "Raw extracted content",
      "method": "static" // "static", "browser", "browser+solver", "pdf", "docx"
    }
  ],
  "cache_status": "miss" // from X-Forage-Cache header: "hit", "miss", "bypass", "disabled"
}
```

**Validation:**
- `urls`: array of non-empty strings, each starting with `http://` or `https://`, length 1–20
- `formats`: all values must be valid (`markdown`, `html`, `raw_html`)
- `timeout`: integer in range 1–120 (validated server-side if not provided)
- `engine`: if provided, must be `trafilatura` or `readability`
- `wait_for`: pass-through CSS selector (used in browser mode); no client-side validation

## Package Structure

Standard Go project layout. All code for this single binary lives under `cmd/forage-mcp/` as a small set of focused files (no `pkg/` or `internal/` at this scope):

```
forage-mcp/
├── cmd/forage-mcp/
│   ├── main.go          // entry point: flags (pflag), config wiring, stdio MCP server start
│   ├── config.go        // env + pflag configuration
│   ├── client.go        // Forage HTTP client (/search, /extract)
│   ├── tools_search.go  // forage_search tool handler
│   ├── tools_extract.go // forage_extract tool handler
│   └── logging.go       // slog setup (stderr, plaintext)
├── go.mod               // module github.com/dark-penguin/forage-mcp, Go 1.25
├── go.sum
├── Makefile             // Build targets: make build, make test
└── README.md
```

The code is split across a few files for clarity but stays within `cmd/forage-mcp/` — the standard idiom for a single small binary. No `pkg/` (nothing is imported as a library) and no `internal/` (not needed at this size).

## Error Handling

Tool-level failures are returned as a Go `error` from the handler; the official MCP SDK sets `isError: true` on the result and includes the error message in the content, so the LLM can see the failure and self-correct. (MCP protocol-level errors — e.g. tool not found — are a separate mechanism handled by the SDK.)

- **Client-side validation errors**: return an error with a clear message (e.g., `"query is required and must be a non-empty string"`)
- **HTTP errors from Forage**: forward Forage's error response body in the returned error (e.g., `forage search failed (HTTP 500): <body>`)
- **Timeouts**: return a timeout error with context (e.g., `request timed out after 120s`)
- **Internal errors**: return a generic message; log full details at debug level (never expose stack traces to clients)

## Logging

All logs go to stderr as **plaintext** (not JSON), using Go's standard `log/slog` with a human-readable text handler.

Verbosity is controlled by:
- `--verbose` / `-v` flag or `LOG_LEVEL=debug` envvar → debug output
- Default: `info` level — incoming requests, outgoing Forage calls, errors, retries
- `--quiet` / `-q` flag or `LOG_LEVEL=warn` envvar → minimal output (errors/warnings only)

Example log line:
```
2026-11-08T20:56:00+04:00 DEBUG  incoming request tool=forage_search params={query:golang mcp limit:5}
```

## Testing Strategy

- Standard Go `testing` package with table-driven tests
- HTTP mocking via `net/http/httptest` for Forage API interactions
- Tests cover: valid inputs, invalid inputs, missing fields, edge cases (empty results, timeouts), parameter validation
- No integration tests against a live Forage instance (external dependency not available in CI)

## Verification

- `make build` compiles the binary successfully
- `./forage-mcp --help` shows all CLI flags and correct defaults
- Tools respond correctly to sample MCP requests against a mock Forage server
- Logs appear on stderr with plaintext format at info level
- All unit tests pass: `go test ./...`
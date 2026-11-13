# Proposal

## Why

AI agents (Claude Desktop, Agent Zero, and any MCP-compatible client) need a reliable way to perform web searches and extract clean page content from the self-hosted [Forage](https://github.com/aldemaroc/forage) service. Today there is no Model Context Protocol (MCP) bridge for Forage's Native API, so agents cannot use its multi-provider search (Google, Bing, Yahoo, DuckDuckGo, Brave) or its hybrid static + browser extraction pipeline without custom glue code per client.

## What Changes

- Introduce a new Go 1.25 binary `forage-mcp` that acts as an MCP server over **stdio** and exposes two tools:
  - `forage_search` — web search via Forage's `/search` endpoint
  - `forage_extract` — clean content extraction via Forage's `/extract` endpoint
- Accept configuration through environment variables (`FORAGE_URL`, `FORAGE_API_KEY`) and CLI flags (`pflag`): `-u/--forage-url`, `-k/--forage-api-key`, `-t/--timeout`, `-v/--verbose`, `-q/--quiet`.
- Perform client-side input validation (query length, URL scheme, engine/format enums, timeout ranges) before forwarding requests to Forage.
- Forward `no_cache: true` as a `Cache-Control: no-cache` header and surface the `X-Forage-Cache` response header as a `cache_status` field on tool results.
- Emit plaintext `slog` logs to stderr with configurable verbosity (debug / info / warn).
- Return tool-level failures as Go errors so the MCP SDK marks the result `isError: true` and the LLM can self-correct.
- Provide a standard Go project layout under `cmd/forage-mcp/`, a `Makefile` (`build`, `test`), and table-driven unit tests using `net/http/httptest` for Forage API interactions.

## Capabilities

### New Capabilities
- `forage-search`: MCP tool that performs web searches through Forage's Native `/search` endpoint, with parameter validation, cache-control handling, and structured result mapping.
- `forage-extract`: MCP tool that extracts clean page content through Forage's Native `/extract` endpoint, with URL/format/engine validation, per-request timeout override, and structured result mapping.
- `mcp-stdio-server`: stdio-based MCP server lifecycle (config loading, logging setup, tool registration, request dispatch) built on the official Go MCP SDK.

### Modified Capabilities

## Impact

- **New code**: entire `forage-mcp` repository content — `cmd/forage-mcp/{main,config,client,tools_search,tools_extract,logging}.go`, `go.mod`, `go.sum`, `Makefile`, `README.md`.
- **Dependencies**: `github.com/modelcontextprotocol/go-sdk`, `github.com/spf13/pflag`; Go standard library (`log/slog`, `net/http`, `encoding/json`).
- **Runtime environment**: requires a reachable Forage instance (URL via env or flag); optional bearer token for authenticated instances.
- **Consumers**: any MCP-compatible client (Claude Desktop, Agent Zero, CLI tools) that can spawn a stdio server process.
- **No changes** to the Forage server itself; this project is a thin client wrapper with no proxy logic or caching of its own.

# Design

## Context

The `forage-mcp` binary is a new, self-contained Go project with no existing codebase. It wraps the Forage Native API (`/search`, `/extract`) behind two MCP tools over stdio. The only external runtime dependency is a reachable Forage instance; all other behavior is local to the process. See `proposal.md` for motivation.

Key constraints shaping the design:
- Must be a single standalone binary (Go, no runtime dependencies).
- Must speak MCP stdio correctly so any compliant client works unchanged.
- Must not proxy, cache, or transform Forage responses beyond structural mapping and header surfacing.
- Configuration must work both in containerized deployments (env vars) and local/CLI usage (flags), with flags taking precedence.

## Goals / Non-Goals

**Goals:**
- Minimal, readable Go codebase that a single maintainer can audit end-to-end.
- Clean separation between config loading, HTTP client, tool handlers, and logging so each concern is independently testable.
- Deterministic error propagation: every failure path produces a structured MCP error result the LLM can act on.
- Table-driven unit tests that cover validation, header mapping, timeout behavior, and error forwarding without requiring a live Forage instance.

**Non-Goals:**
- No browser control, headless rendering, or proxy logic inside the binary (handled by Forage).
- No authentication token management beyond passing a bearer token through to Forage.
- No multi-instance process management, systemd units, or supervisor integration.
- No Firecrawl-compatible API mode; only the Native Hermes-compatible endpoints are in scope.

## Decisions

### Language and SDK

**Decision:** Go 1.24 with the official `github.com/modelcontextprotocol/go-sdk` for MCP server scaffolding and `github.com/spf13/pflag` for CLI flags.

**Rationale:**
- Go produces a single static binary with no runtime dependencies, ideal for containerized or CLI deployment.
- The official Go MCP SDK handles stdio transport framing, JSON-RPC marshaling, tool registration, and `isError` result construction, eliminating hand-rolled protocol code.
- `pflag` is POSIX-compatible, well-tested, and matches the flag conventions of similar tools in the ecosystem. It supports both short (`-u`) and long (`--forage-url`) forms with env-var fallback via a simple two-step load.

**Alternatives considered:**
- *Python MCP SDK*: would require a Python runtime or PyInstaller bundling, increasing deployment friction and binary size.
- *Rust MCP SDK*: stronger performance but a smaller ecosystem for this use case; Go's stdlib HTTP client and `slog` cover all needs without extra dependencies.
- *Hand-rolled JSON-RPC over stdio*: would duplicate what the official SDK already provides correctly, including edge cases in framing and error marshaling.

### Project layout

**Decision:** All code lives under `cmd/forage-mcp/` as a flat set of focused files. No `pkg/` or `internal/` directories.

**Rationale:**
- This is a single small binary; nothing is imported as a library by other Go packages, so `pkg/` would be misleading.
- `internal/` adds path constraints that are unnecessary at this scale and would complicate future extraction if the project ever grows.
- A flat layout under `cmd/forage-mcp/` is the standard Go idiom for a single-binary project and keeps the file count visible in one directory listing.

**File responsibilities:**
| File | Responsibility |
|---|---|
| `main.go` | Entry point: parse flags, load config, set up logging, register tools with MCP SDK, start stdio server loop. |
| `config.go` | Define `Config` struct; implement `LoadConfig()` that reads env vars then applies pflag overrides; validate that a Forage URL is present. |
| `client.go` | Define `ForageClient` wrapping `*http.Client`; implement `Search(ctx, params)` and `Extract(ctx, params)` methods that build the HTTP request (headers, body, timeout), send it, read `X-Forage-Cache`, and decode the JSON response into typed structs. |
| `tools_search.go` | Implement the `forage_search` tool handler: validate parameters, call `client.Search`, map the result to the MCP tool result schema. |
| `tools_extract.go` | Implement the `forage_extract` tool handler: validate parameters (including per-request timeout override), call `client.Extract`, map the result to the MCP tool result schema. |
| `logging.go` | Set up `slog` with a text handler writing to stderr; expose `InitLogging(level)` called from `main.go`. |

**Alternatives considered:**
- *Separate `internal/forage/` package for the HTTP client*: would be appropriate if the client were reused by other binaries, but at this scope it adds indirection without benefit.
- *Single `main.go` file*: would exceed ~800 lines and mix concerns, making targeted testing harder.

### Configuration precedence

**Decision:** Environment variables are read first; CLI flags override them. The effective config is computed once at startup in `LoadConfig()`.

**Rationale:**
- Env vars are the natural mechanism for containerized deployments (Docker secrets, K8s env).
- Flags provide a convenient local/CLI override without requiring env setup.
- Computing the final config once avoids per-request re-evaluation and makes the precedence rule easy to test.

**Implementation note:** `pflag` does not natively read env vars. The pattern is: read `os.Getenv("FORAGE_URL")` into a default, pass it as the pflag default value, then use the flag's final value after parsing. This gives flags automatic precedence over env vars with no extra logic.

### HTTP client and timeout handling

**Decision:** A single `*http.Client` is created at startup with the global timeout. For `forage_extract`, a per-request `context.WithTimeout` is used when the `timeout` parameter is provided, overriding the client-level timeout for that request only.

**Rationale:**
- The global timeout applies to all requests by default, matching the spec requirement.
- Per-request timeout override is implemented via context cancellation rather than creating a new `http.Client`, which would be wasteful and complicate connection pooling.
- The effective timeout for a request is: per-request `timeout` if provided and valid, otherwise the global timeout. This is resolved in the tool handler before calling the client.

**Alternatives considered:**
- *Creating a new `http.Client` per request with the desired timeout*: would discard connection pooling and add allocation overhead.
- *Using only the global timeout and ignoring the per-request parameter*: would violate the spec requirement for `forage_extract`.

### Logging architecture

**Decision:** `slog` with a text handler writing to `os.Stderr`. Log level is resolved from (in priority order): `-v` flag → `-q` flag → `LOG_LEVEL` env var → default `info`. The logger is initialized once in `main.go` before the MCP server starts.

**Rationale:**
- `slog` is in the Go standard library, requires no dependencies, and produces clean plaintext output by default with its text handler.
- Writing to stderr keeps stdout reserved for MCP protocol messages, which is a hard requirement of the stdio transport.
- The priority order (flag > env > default) matches user expectation: explicit CLI flags are the most specific signal.

**Alternatives considered:**
- *JSON log format*: would be harder to read during local development and debugging; plaintext is preferred for a developer-facing tool.
- *Third-party logging library (zap, zerolog)*: would add a dependency for functionality that `slog` already provides adequately at this scale.

### Error handling strategy

**Decision:** Tool handlers return Go `error` values. The MCP SDK converts these to MCP results with `isError: true`. Error messages are constructed at the point of failure with enough context for the LLM to self-correct, but without internal implementation details.

**Rationale:**
- The official MCP SDK's tool handler signature returns `(result, error)`, so returning errors is the idiomatic pattern and requires no custom marshaling.
- Structured error messages (e.g., `"query is required and must be a non-empty string"`, `"forage search failed (HTTP 500): <body>"`) give the LLM actionable information.
- Internal errors (unexpected panics, encoding failures) are logged at debug level with full detail but return a generic message to the client, preventing information leakage.

**Alternatives considered:**
- *Returning error details in the result body instead of using `isError`*: would violate MCP protocol conventions and make it harder for clients to distinguish success from failure programmatically.
- *Custom error types with structured fields*: would be useful if errors were consumed by multiple layers, but at this scale a well-formatted string is sufficient.

### Test strategy

**Decision:** Table-driven Go tests using `net/http/httptest` to mock the Forage API. Each test file mirrors its source file (`config_test.go`, `client_test.go`, `tools_search_test.go`, `tools_extract_test.go`).

**Rationale:**
- `httptest.NewServer` provides a real HTTP server on a local port, so the HTTP client code is exercised end-to-end without external dependencies.
- Table-driven tests keep validation edge cases (empty query, out-of-range limit, invalid engine, etc.) concise and easy to extend.
- Mirroring source files keeps related test logic co-located and makes it easy to find tests for a given function.

**Alternatives considered:**
- *Integration tests against a live Forage instance*: would require a running Forage server, making CI flaky and slow; unit tests with `httptest` cover the same contract without that dependency.
- *Mocking at the HTTP transport level (`http.RoundTripper`)*: would work but is less realistic than `httptest` for testing header handling and response decoding.

## Risks / Trade-offs

- **MCP SDK API stability**: The official Go MCP SDK is relatively new. Breaking changes in the SDK could require code adjustments. → Mitigation: pin the SDK version in `go.mod`; the surface area used (server creation, tool registration, stdio transport) is small and unlikely to change drastically.

- **Forage API contract drift**: If Forage changes its Native API response shape or header names, the mapping logic will break silently or produce malformed results. → Mitigation: unit tests assert the exact response shape; a future integration test against a pinned Forage version could catch drift earlier.

- **Single-binary deployment assumes stdio**: Clients that expect HTTP/SSE transport for MCP will not work with this server. → This is an intentional scope decision documented in the proposal; no mitigation needed, but it should be clear in the README.

- **No retry logic**: Transient network failures or Forage 5xx responses are returned to the client as errors without retry. → Mitigation: the LLM can retry by calling the tool again; adding automatic retries would mask real failures and complicate timeout accounting. If retries become necessary, they should be added at the client (LLM) level, not in this binary.

- **Timeout override via context**: Using `context.WithTimeout` for per-request overrides means the global client timeout still applies as a hard ceiling if the context is not properly propagated. → Mitigation: the tool handler sets the context timeout to the effective value (per-request or global), and the `http.Client` uses that context; tests verify that a short per-request timeout actually cancels the request.

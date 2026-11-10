# Code Review: forage-mcp

*Reviewed: 2026-11-11*

## Overview

A clean, well-structured MCP server in Go that bridges AI agents to the Forage web search/extract service. The code is idiomatic, well-documented, and appropriately scoped for a single-binary project. Overall quality is **good** with a few issues worth addressing.

---

## 🐛 Bug

### 1. Timeout detection is unreliable (`client.go:113-114`)

```go
if ctx.Err() == context.DeadlineExceeded {
    return nil, "", fmt.Errorf("request timed out after %d seconds", int(c.httpClient.Timeout.Seconds()))
}
```

This checks the **parent** context for deadline expiry, but the `http.Client.Timeout` fires independently of the parent context. When the client's own timeout expires, `ctx.Err()` returns `nil` (the parent context is still valid), so this branch is never taken for client-level timeouts. The error falls through to the generic `forage request failed: %w` path.

**Fix:** Check the error type instead:

```go
if err != nil {
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Timeout() {
        return nil, "", fmt.Errorf("request timed out after %d seconds", int(c.httpClient.Timeout.Seconds()))
    }
    if ctx.Err() == context.DeadlineExceeded {
        return nil, "", fmt.Errorf("request cancelled (deadline exceeded)")
    }
    return nil, "", fmt.Errorf("forage request failed: %w", err)
}
```

---

## ⚠️ Inconsistencies

### 2. Go version mismatch

| Location | Version |
|---|---|
| `README.md` line 19 | `Go 1.24+` |
| `project.md` lines 18, 34 | `Go 1.24` |
| `go.mod` line 3 | `go 1.25.0` |

Pick one and align all three. If you need Go 1.25 features, update the docs; if not, lower the go.mod directive.

---

## 🏗️ Design Concerns

### 3. Package-level globals (`main.go:12-14`)

```go
var (
    serverConfig *Config
    forageClient *ForageClient
)
```

This works for the current scope but creates two problems:

- **Test fragility**: Every handler test must swap and restore these globals (see `tools_search_test.go:69-76`). If a test panics mid-swap, state leaks into other tests.
- **No concurrency safety**: If the MCP SDK ever dispatches tool calls concurrently, two goroutines could read/write these vars simultaneously.

**Suggestion** (optional, not urgent): Pass `*ForageClient` and `*Config` through a small struct captured in a closure when registering handlers:

```go
type app struct {
    cfg    *Config
    client *ForageClient
}

func (a *app) searchHandler(ctx context.Context, req *mcp.CallToolRequest, input searchToolArgs) (...) {
    result, err := a.client.Search(ctx, params, input.NoCache)
    // ...
}
```

This eliminates the globals entirely and makes each handler test self-contained.

### 4. No response body size limit (`client.go:122`)

```go
respBody, err := io.ReadAll(resp.Body)
```

A misbehaving or malicious Forage instance could return an arbitrarily large body, causing OOM. Consider capping it:

```go
respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB cap
```

For an MCP server proxying web content, 50 MB is generous but safe.

### 5. No startup health check

The server starts and immediately begins accepting MCP requests without verifying the Forage instance is reachable. If `FORAGE_URL` points to a dead service, every tool call will fail with a connection error. A one-shot `GET /health` at startup (with a short timeout) would give immediate feedback:

```go
if err := forageClient.Health(ctx); err != nil {
    slog.Warn("forage instance not reachable at startup", "url", cfg.ForageURL, "error", err)
}
```

Non-fatal — just a warning — since Forage might come up later.

---

## 🧪 Testing

### 6. `config_test.go` env/args restoration is panic-unsafe

Lines 102-125 manually save/restore `os.Args` and env vars. If `LoadConfig()` panics, the restore code never runs, polluting subsequent tests.

**Fix:** Use `t.Cleanup()`:

```go
t.Run(tt.name, func(t *testing.T) {
    origArgs := os.Args
    t.Cleanup(func() { os.Args = origArgs })
    // ... set env vars with t.Cleanup for each
})
```

### 7. Missing test coverage

- **Timeout path**: No test exercises the (buggy) timeout detection in `client.go`. Add a test with a server that sleeps longer than the client timeout.
- **Large response body**: No test for the `io.ReadAll` path with a large payload.
- **Concurrent handler calls**: No test verifies behavior under parallel tool invocations (relevant if the globals issue is fixed).

---

## 🔒 Security

### 8. API key via CLI flag visible in process list

`-k my-secret-key` is visible via `ps aux`. This is inherent to the flag-based approach and mitigated by the env var alternative, but worth documenting in the README:

> **Note:** Prefer `FORAGE_API_KEY` env var over `-k` flag to avoid exposing the key in process listings.

### 9. No TLS verification bypass (good)

The HTTP client uses default `tls.Config`, so certificate verification is enforced. This is correct — don't add an insecure mode.

---

## 📝 Documentation

### 10. README missing troubleshooting section

Add a short section covering:

- "Connection refused" → verify Forage is running and `FORAGE_URL` is correct
- "HTTP 401" → API key mismatch or auth not enabled on Forage
- "Timeout" → increase `-t` or check Forage browser pool

### 11. No version injection mechanism

The version is hardcoded as `"v1.0.0"` in `main.go:32`. For a binary that will be distributed, consider ldflags injection:

```makefile
LDFLAGS := -X main.version=$(VERSION)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/forage-mcp
```

---

## ✅ What's Done Well

- **Clean separation of concerns**: config, client, tools, and logging are each in their own file with clear responsibilities.
- **Thorough input validation**: Both tools validate all parameters with clear error messages before hitting the network.
- **Excellent end-to-end test** (`main_test.go`): Tests the full MCP protocol flow — initialize, list tools, call both tools, verify structured content, and test error paths. This is the most valuable test in the project.
- **Proper stdio discipline**: All logging goes to stderr; stdout is reserved for MCP protocol. Critical for stdio transport correctness.
- **Good use of `slog`**: Structured logging with appropriate levels (debug for request/response details, info for lifecycle, warn for HTTP errors).
- **Pointer types for optional numerics** (`*int` for limit/timeout): Correctly distinguishes "not provided" from zero value.
- **Project documentation** (`project.md`, `forage_api_docs.md`): Design decisions are well-reasoned and documented. The API mapping section in project.md is especially helpful.
- **Minimal dependencies**: Only two direct deps (MCP SDK + pflag). Appropriate for the scope.

---

## Summary

| Category | Rating |
|---|---|
| Architecture | ✅ Good — appropriate for scope, minor global-var smell |
| Code Quality | ✅ Good — idiomatic Go, clean error handling |
| Correctness | ⚠️ One real bug (timeout detection), one inconsistency (Go version) |
| Security | ✅ Good — no issues found, minor CLI flag note |
| Testing | ✅ Good coverage, panic-unsafe env restoration, missing timeout test |
| Documentation | ✅ Excellent — README + design doc + API reference |
| Build/Tooling | ⚠️ Minimal Makefile, no vet/lint/fmt targets |

**Priority fixes:** #1 (timeout bug), #2 (version mismatch). The rest are improvements that can be addressed incrementally.

# Code Review: forage-mcp (Round 2)

*Reviewed: 2026-11-11*

## Overview

This is the second code review of the `forage-mcp` project. The codebase has been significantly improved since the first review — most previously identified issues have been addressed. The project is now a well-structured, idiomatic Go MCP server with solid test coverage and clean architecture.

**Overall assessment: Production-ready with minor polish items.**

---

## ✅ Changes Since Review 1

| Issue (Review 1) | Status |
|---|---|
| #1 Timeout detection bug | ✅ Fixed — `net.Error` check now precedes `ctx.Err()` check (`client.go:115-118`) |
| #2 Go version mismatch | ✅ Fixed — all docs now say `Go 1.25+`, matching `go.mod` |
| #3 Package-level globals | ✅ Fixed — replaced with `app` struct carrying `cfg` and `client` (`main.go:13-16`) |
| #4 No response body size limit | ✅ Fixed — `io.LimitReader(resp.Body, 50<<20)` at `client.go:128` |
| #5 No startup health check | ✅ Fixed — `GET /health` with 5s timeout, non-fatal warning (`main.go:35-37`) |
| #6 Panic-unsafe env restoration | ✅ Fixed — `t.Cleanup()` used throughout `config_test.go` |
| #7 Missing test coverage | ✅ Partially — concurrency test added; timeout path still untested (see below) |
| #8 API key CLI flag visibility | ✅ Fixed — README note added at line 46 |
| #10 Missing troubleshooting section | ✅ Fixed — README lines 177-181 |
| #11 No version injection | ✅ Fixed — `Makefile` uses `-X main.version=$(VERSION)` ldflags |

All major issues from the first review have been resolved. The remaining items below are new observations and minor polish.

---

## 🐛 Bugs / Correctness Issues

### 1. Timeout error reports wrong duration (`client.go:117`)

```go
if errors.As(err, &netErr) && netErr.Timeout() {
    return nil, "", fmt.Errorf("request timed out after %d seconds", int(c.httpClient.Timeout.Seconds()))
}
```

This reports the **client-level** timeout duration. However, when a per-request `context.WithTimeout` is used (as in `tools_extract.go:89`), the actual deadline that fired may be shorter than the client's global timeout. The error message would say "timed out after 120 seconds" when the effective timeout was actually 45 seconds.

**Fix:** Check the context first, and fall back to the client timeout only if the context is still valid:

```go
if ctx.Err() == context.DeadlineExceeded {
    return nil, "", fmt.Errorf("request timed out (context deadline exceeded)")
}
var netErr net.Error
if errors.As(err, &netErr) && netErr.Timeout() {
    return nil, "", fmt.Errorf("request timed out after %d seconds", int(c.httpClient.Timeout.Seconds()))
}
```

Or better, pass the effective timeout into `doRequest` so the error message is always accurate.

### 2. Health check uses client-level timeout instead of its own (`client.go:147-148`)

```go
func (c *ForageClient) Health(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
```

The context timeout is set to 5 seconds, but the `http.Client` has a global timeout of 120 seconds. The `http.Client.Timeout` takes precedence over the context deadline when it's shorter — actually no, in Go the **context** deadline and the **client** timeout are independent; whichever fires first cancels the request. So this is actually correct: the 5-second context will fire before the 120-second client timeout. No bug here, but worth a comment for clarity.

### 3. `forage_search` has no per-request timeout context (`tools_search.go:72`)

```go
result, err := a.client.Search(ctx, searchParams, input.NoCache)
```

The search handler passes the raw incoming `ctx` without wrapping it in `context.WithTimeout`. This means the only timeout protection is the `http.Client.Timeout` (global). While this works, it's inconsistent with the extract handler which explicitly sets a context timeout. If the MCP SDK ever changes how it manages request contexts, the search path would lose its timeout guarantee.

**Suggestion:** Add an explicit context timeout for consistency:

```go
effectiveTimeout := a.cfg.TimeoutSeconds
ctx, cancel := context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
defer cancel()
```

---

## ⚠️ Design & Code Quality

### 4. `doRequest` returns raw JSON instead of typed structs (`client.go:88`)

The `doRequest` helper returns `json.RawMessage`, and each caller (`Search`, `Extract`) unmarshals it separately. This is fine for the current two-endpoint scope, but if more endpoints are added, the pattern will require a new unmarshal block per method. A generic helper or type parameter could reduce duplication:

```go
func (c *ForageClient) doRequest[T any](ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) (*T, string, error)
```

Not urgent — the current approach is clear and testable.

### 5. `Health` method doesn't use `doRequest` (`client.go:146-169`)

The health check duplicates request-building logic (header setup, status checking) instead of reusing `doRequest`. This is acceptable since it's a GET with no body and different semantics (no JSON decode), but if the auth header logic changes, both places need updating. Consider extracting just the header-setup into a small helper.

### 6. No request logging at info level (`client.go:111`)

The design doc says "Default: `info` level — incoming requests, outgoing Forage calls, errors" but the outbound Forage request is only logged at `debug` level:

```go
slog.Debug("sending forage request", "method", method, "path", path)
```

At the default info level, you won't see which Forage endpoints are being called. Consider promoting to `info` or adding an info-level log in the tool handlers (which they do — `tools_search.go:59`, `tools_extract.go:77` — so this is actually fine).

**Verdict:** No issue — the tool handlers log at info level, which satisfies the spec.

### 7. Missing `logging.go` file

The design doc and tasks reference a `logging.go` file with an `InitLogging(level)` function, but the logging setup is inlined in `main.go:25-28`. This is a minor deviation from the documented structure — the code works identically, but the project layout in `project.md:164` lists `logging.go` as a planned file.

**Suggestion:** Either create `logging.go` to match the docs, or update the docs to reflect the actual structure. The current inline approach is perfectly fine for this scope.

---

## 🧪 Testing

### 8. No test for timeout/cancellation path

There's no test that exercises the timeout error path in `doRequest`. A test with a server that sleeps longer than the client timeout would verify:
- The `net.Error` timeout branch is hit
- The error message includes the correct duration
- Context cancellation produces the right error

```go
func TestDoRequestTimeout(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(2 * time.Second)
    }))
    defer server.Close()

    client := NewForageClient(server.URL, "", 1) // 1s timeout
    _, _, err := client.doRequest(context.Background(), http.MethodPost, "/search", nil, nil)
    if err == nil {
        t.Fatal("expected timeout error")
    }
    if !strings.Contains(err.Error(), "timed out") {
        t.Errorf("error should mention timeout: %v", err)
    }
}
```

### 9. No test for large response body (50 MB cap)

The `io.LimitReader` cap at `client.go:128` has no test. A test with a server returning >50 MB would verify the cap works and doesn't cause an error (it just truncates). Lower priority since it's a safety net.

### 10. Concurrency test is good but could be stronger (`concurrency_test.go`)

The existing test fires 20 concurrent search + 20 concurrent extract calls, which is solid. One improvement: add a `sync.Mutex`-protected counter in the mock server to verify all requests actually completed (not just that no errors were returned).

### 11. End-to-end test doesn't verify `cache_status` (`main_test.go`)

The e2e test checks `success` and `data.web` but doesn't assert on `cache_status`. The mock server sets `X-Forage-Cache: miss` for search and `hit` for extract — these should be verified:

```go
if cacheStatus, ok := searchResp["cache_status"].(string); !ok || cacheStatus != "miss" {
    t.Errorf("expected cache_status=miss, got %v", searchResp["cache_status"])
}
```

---

## 🔒 Security

### 12. No issues found

- TLS verification is enforced (default `http.Client` config) ✅
- API key is only sent when configured ✅
- Response body is capped at 50 MB ✅
- No sensitive data in logs (query is logged at info, but that's the user's own search term) ✅
- No path traversal or injection risks (all input is validated before use) ✅

---

## 📝 Documentation & Project Hygiene

### 13. `project.md` still references `logging.go`

The package structure section (`project.md:164`) lists `logging.go` as a file, but it doesn't exist. Update the docs or create the file.

### 14. `design.md` says Go 1.24, `go.mod` says 1.25

`design.md:31` states "Go 1.24 with the official..." but `go.mod` specifies `go 1.25.0`. Minor inconsistency in the design doc.

### 15. `.README.md.swp` file present

There's a Vim swap file (`.README.md.swp`) in the project root. This should be in `.gitignore` or deleted.

---

## ✅ What's Done Well

- **Architecture is clean**: The `app` struct pattern eliminates global state, makes handlers testable, and is concurrency-safe.
- **Comprehensive validation**: Both tools validate all parameters with clear, actionable error messages before hitting the network.
- **Excellent end-to-end test** (`main_test.go`): Full MCP protocol flow — initialize, list tools, call both tools, verify structured content, and error paths. This is the most valuable test in the project.
- **Proper stdio discipline**: All logging goes to stderr; stdout is reserved for MCP protocol messages.
- **Good use of `slog`**: Structured logging with appropriate levels — debug for wire details, info for lifecycle, warn for HTTP errors.
- **Pointer types for optional numerics** (`*int` for limit/timeout): Correctly distinguishes "not provided" from zero value.
- **Concurrency test**: 40 parallel handler calls verify thread-safety of the `app` struct pattern.
- **Health check at startup**: Non-fatal warning gives immediate feedback if Forage is unreachable.
- **Response body cap**: 50 MB limit prevents OOM from misbehaving backends.
- **Minimal dependencies**: Only two direct deps (MCP SDK + pflag). Appropriate for the scope.
- **All Review 1 issues addressed**: The codebase has been significantly improved since the first review.

---

## Summary

| Category | Rating |
|---|---|
| Architecture | ✅ Excellent — `app` struct, clean separation, concurrency-safe |
| Code Quality | ✅ Excellent — idiomatic Go, clear error handling, good documentation |
| Correctness | ⚠️ Minor — timeout error reports wrong duration in per-request override case |
| Security | ✅ No issues found |
| Testing | ✅ Good — e2e, unit, validation, concurrency; missing timeout path test |
| Documentation | ✅ Good — README + design doc + API reference; minor inconsistencies |
| Build/Tooling | ✅ Good — Makefile with version injection, clean targets |

**Priority items:**
1. Fix timeout error message to report the effective (not client-level) duration — low effort, improves debuggability
2. Add a timeout/cancellation test for `doRequest` — fills the last meaningful coverage gap
3. Clean up `.README.md.swp` and align `design.md` Go version reference

The codebase is in excellent shape and ready for production use.

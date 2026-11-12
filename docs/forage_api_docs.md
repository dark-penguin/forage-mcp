# Forage MCP Server Preparation

## Overview

**Forage** is a self-hosted web search & extract service that provides:
- **Web Search**: Via its own SERP engines (Google/Bing/Yahoo/DuckDuckGo) or SearXNG
- **Web Extract**: Hybrid static + browser extraction with anti-bot coverage
- **Firecrawl Compatibility**: Drop-in replacement for Firecrawl's API

**Repository**: https://github.com/aldemaroc/forage
**License**: GPL v3
**Stack**: Python 3.12+, FastAPI, Playwright/Patchright/Scrapling (Chromium)

---

## API Endpoints

All endpoints run on the same port (default `3672`).

### Authentication

Optional Bearer token auth via `Authorization: Bearer <key>`. Enabled via `auth.enabled` in config. Keys set via `FORAGE_API_KEYS` env var.

**Bypass cache**: Send `Cache-Control: no-cache` header to skip in-memory caching.

---

## Native API (Hermes-compatible)

### `GET /health`

Liveness probe. Always open (no auth required).

**Response**:
```json
{
  "status": "ok",
  "service": "forage",
  "version": "1.0.0",
  "config_source": "...",
  "browser_engine": "scrapling",
  "search_provider": "forage",
  "firecrawl_prefixes": ["/v1", "/v2"]
}
```

---

### `POST /search`

Search via configured provider (Forage's own SERP or SearXNG).

**Request Body**:
```json
{
  "query": "string (1-500 chars)",
  "limit": 5,          // integer, 1-50 (default: 5)
  "language": "pt-BR", // string (default: configured `search.default_lang`)
  "engines": ["google", "bing"] // array of: google, bing, yahoo, duckduckgo/ddg, brave
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "web": [
      {
        "title": "...",
        "url": "https://...",
        "description": "...",
        "position": 1
      }
    ]
  }
}
```

**Headers**:
- `X-Forage-Cache`: `hit | miss | bypass | disabled`

---

### `POST /extract`

Extract URLs using hybrid strategy (static → browser fallback).

**Request Body**:
```json
{
  "urls": ["https://example.com"],     // array, 1-20 URLs
  "formats": ["markdown"],            // "markdown" (default) | "html" | "raw_html"
  "only_main_content": true,           // bool, default: true
  "force_render": false,               // bool, force browser mode
  "wait_for": null,                    // string, CSS selector to wait for (browser)
  "timeout": 30,                       // integer seconds, 1-120
  "engine": null                       // "trafilatura" | "readability"
}
```

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "url": "https://...",
      "title": "...",
      "content": "clean markdown text...",
      "raw_content": "clean markdown or raw HTML",
      "method": "static"  // "static" | "browser" | "browser+solver" | "pdf" | "docx" | ...
    }
  ]
}
```

**Headers**:
- `X-Forage-Cache`: `hit | miss | bypass | disabled`

---

### `POST /admin/cache/purge`

Clear in-memory caches (auth-gated).

**Response**:
```json
{ "cleared": 42 }
```

---

## Firecrawl-Compatible API

Served under `/v1` and `/v2` prefixes on the same port.

### `POST /v1/scrape` (and `/v2/scrape`)

Single URL scraping with Firecrawl response envelope.

**Request Body**:
```json
{
  "url": "https://example.com",         // required, http/https only
  "formats": ["markdown"],             // "markdown" (default) | "html" | "rawHtml"
  "onlyMainContent": true,              // bool, default: true
  "timeout": 30000                      // integer milliseconds → converted to seconds (1-120s)
}
```

**Ignored fields**: All other Firecrawl fields (`actions`, `headers`, `proxy`, `waitFor`, `maxAge`, `includeTags`, `screenshotOptions`, etc.) are accepted without error and listed in `data.warning`.

**Success Response**:
```json
{
  "success": true,
  "data": {
    "markdown": "# Title\
\
Content...",
    "html": "<html>...",                // if requested
    "rawHtml": "<html>...",             // same as html for Forage
    "metadata": {
      "title": "Example Domain",
      "sourceURL": "https://example.com",
      "url": "https://example.com",
      "statusCode": 200,
      "cacheState": "miss"
    },
    "warning": "..."                  // only if unsupported options were ignored
  }
}
```

**Error Response**:
```json
{
  "success": false,
  "code": "SCRAPE_ALL_ENGINES_FAILED",
  "error": "Blocked by anti-bot challenge (Cloudflare or similar)"
}
```

**Error Codes**:
| Code | HTTP | Description |
|---|---|---|
| `BAD_REQUEST` | 400 | Missing or non-http(s) `url` |
| `SCRAPE_ALL_ENGINES_FAILED` | 500 | No engine could extract / no usable text |
| `SCRAPE_TIMEOUT` | 408 | Extraction timed out |
| `UNKNOWN_ERROR` | 500 | Other server-side failure |

---

### `POST /v1/search` (and `/v2/search`)

Search with Firecrawl response envelope.

**Request Body**:
```json
{
  "query": "proxmox server",  // required
  "limit": 5,                 // integer, 1-100 (default: 5)
  "lang": "pt-BR",           // v1 field name for language
  "language": "pt-BR",       // alternative field name
  "scrapeOptions": {...}      // not supported → warning only
}
```

**v1 Response** (flat array):
```json
{
  "success": true,
  "data": [
    { "url": "...", "title": "...", "description": "...", "position": 1 }
  ],
  "id": "5f1c2f2e-..."
}
```

**v2 Response** (grouped):
```json
{
  "success": true,
  "data": {
    "web": [ { "url": "...", ... } ]
  },
  "id": "5f1c2f2e-..."
}
```

**Ignored fields**: `tbs`, `country`, `location`, `filter`, etc. are accepted without error and listed in `data.warning`.

---

## Configuration (config.yaml)

### `server`
| Key | Default | Description |
|---|---|---|
| `host` | `0.0.0.0` | Bind address |
| `port` | `3672` | HTTP port |
| `workers` | `2` | Uvicorn workers |
| `log_level` | `info` | `debug`/`info`/`warning`/`error` |

### `cache`
| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Master switch |
| `max_entries` | `500` | Global LRU cap |
| `search.enabled` | `true` | Cache search results |
| `search.ttl` | `300` | Search TTL (5 min) |
| `extract.enabled` | `false` | Extract caching off by default |
| `extract.ttl` | `3600` | Extract TTL (1 h) |

### `search`
| Key | Default | Description |
|---|---|---|
| `provider` | `forage` | `forage` (own SERP) or `searxng` |
| `searxng_url` | `http://searxng:8080` | SearXNG instance URL |
| `default_lang` | `pt-BR` | Default language |
| `engines` | `[google, bing, duckduckgo]` | Ordered SERP engines |
| `timeout` | `15` | Timeout per request (searxng mode) |
| `serp_timeout` | `20` | Seconds per SERP render (forage mode) |

### `extract`
| Key | Default | Description |
|---|---|---|
| `timeout` | `30` | Total seconds budget per URL (1-120) |
| `max_content_chars` | `100000` | Content size cap |
| `only_main_content` | `true` | Strip nav/ads/footer |
| `engine` | `readability` | `readability` or `trafilatura` |
| `user_agent` | `ForageBot/0.1` | Static fetch UA |
| `respect_robots` | `false` | Honor robots.txt |
| `force_render` | `false` | Always use browser |
| `wait_for` | `null` | CSS selector to wait for |
| `min_content_chars` | `200` | Fallback to browser if less text |
| `raw_content_markdown` | `true` | Firecrawl-style raw content |
| `prefer_markdown` | `true` | Negotiate markdown Accept header |
| `domain_overrides` | `{}` | Per-domain overrides |

### `browser`
| Key | Default | Description |
|---|---|---|
| `engine` | `scrapling` | `scrapling`/`playwright`/`patchright`/`chrome-local` |
| `cdp_url` | `""` | CDP endpoint for chrome-local |
| `min_idle` | `1` | Warm instances at boot |
| `max_instances` | `5` | Pool ceiling / concurrency bound |
| `idle_timeout` | `60` | Idle instance lifetime (seconds) |
| `headless` | `true` | Extract browser headless |
| `stealth` | `true` | Hide automation signals |
| `fallback_solver` | `true` | Retry with Scrapling solver on anti-bot |

### `browser.search` (forage mode only)
| Key | Default | Description |
|---|---|---|
| `headless` | `false` | Search browser headless (Xvfb if false) |
| `local` | `true` | Use container browser |
| `humanize` | `true` | Mouse movement between SERPs |
| `min_interval` | `2.5` | Seconds between renders (process-wide) |
| `retries` | `2` | Retries on anti-bot for primary engine |

### `auth`
| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Require Bearer auth |

---

## Environment Variables

| Variable | Purpose |
|---|---|
| `FORAGE_API_KEYS` | Comma-separated Bearer keys (auth) |
| `FORAGE_CONFIG` | Config file path inside container |
| `TZ` | Container timezone |

---

## Not Implemented (by design)

Firecrawl endpoints NOT supported:
- `/v1/crawl`, `/v1/map`, `/v1/batch/scrape`, `/v1/extract`, `/v1/parse` → 404

Formats NOT supported in scrape:
- `json`, `extract`, `summary` (require LLM)
- `screenshot` (no screenshot pipeline)
- `links`, `images` (not kept by extraction)
- `actions` (click/type/wait scripts) → ignored with warning

---

## Key Architectural Notes for MCP Implementation

1. **Two API flavors**: Native (`/search`, `/extract`) and Firecrawl-compatible (`/v1/scrape`, `/v1/search`). MCP should prefer native APIs since they're simpler.
2. **Auth is optional but recommended**: The service supports Bearer token auth via `auth.enabled` + `FORAGE_API_KEYS`.
3. **Search provider selection**: `forage` (built-in SERP) or `searxng` (external SearXNG instance).
4. **Browser engines**: Scrapling is default (best anti-bot), but Playwright/Patchright/Chrome-local are alternatives.
5. **Hybrid extraction**: Automatically tries static first, falls back to browser when needed (SPA detection, anti-bot, low text yield).
6. **In-memory caching**: Search cached by default (5min TTL), extract cached off by default. Can be bypassed per-request with `Cache-Control: no-cache`.
7. **Error handling**: Firecrawl-compatible error codes (`SCRAPE_ALL_ENGINES_FAILED`, etc.) enable fallback logic for clients.
8. **Config-driven**: All behavior controlled via `config.yaml` (bind-mounted read-only). No hot reload — restart container to apply changes.

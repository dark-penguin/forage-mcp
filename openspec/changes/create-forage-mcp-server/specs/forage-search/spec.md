# Spec Delta

## Purpose

Lets MCP clients perform web searches through a Forage backend and receive structured, cache-aware search results.

## ADDED Requirements

### Requirement: Search tool is exposed to MCP clients
The system SHALL expose an MCP tool named `forage_search` that accepts a search query and optional parameters, and returns structured web search results from the Forage backend.

#### Scenario: Client invokes search with a valid query
- **WHEN** an MCP client calls `forage_search` with `query` set to a non-empty string of at most 500 characters
- **THEN** the system forwards the request to the Forage `/search` endpoint and returns a result containing `success: true`, a `data.web` array of search hits, and a `cache_status` field

#### Scenario: Client invokes search without optional parameters
- **WHEN** an MCP client calls `forage_search` with only `query` provided
- **THEN** the system uses Forage's configured defaults for `limit`, `language`, and `engines`, and returns results as if those defaults were applied

### Requirement: Search result shape
The system SHALL return search results in a stable JSON structure containing a top-level `success` boolean, a `data.web` array of hit objects with `title`, `url`, `description`, and `position` fields, and a top-level `cache_status` string.

#### Scenario: Forage returns hits
- **WHEN** the Forage `/search` endpoint responds successfully with one or more web results
- **THEN** each result object in `data.web` contains `title`, `url`, `description`, and a 1-based integer `position`

#### Scenario: Forage returns no hits
- **WHEN** the Forage `/search` endpoint responds successfully with an empty result set
- **THEN** the system returns `success: true` and `data.web` as an empty array, not an error

### Requirement: Cache status is surfaced from response header
The system SHALL read the `X-Forage-Cache` HTTP response header on every search request and expose its value (`hit`, `miss`, `bypass`, or `disabled`) as the top-level `cache_status` field in the tool result.

#### Scenario: Forage responds with a cache hit
- **WHEN** the Forage `/search` response includes header `X-Forage-Cache: hit`
- **THEN** the returned tool result contains `cache_status: "hit"`

#### Scenario: Forage omits the cache header
- **WHEN** the Forage `/search` response does not include an `X-Forage-Cache` header
- **THEN** the returned tool result omits `cache_status` or sets it to a clearly empty value, without failing the request

### Requirement: no_cache flag is forwarded as Cache-Control header
The system SHALL send a `Cache-Control: no-cache` HTTP request header to Forage when the `no_cache` parameter is `true`, and SHALL NOT send that header when `no_cache` is `false` or omitted.

#### Scenario: Client requests cache bypass
- **WHEN** an MCP client calls `forage_search` with `no_cache: true`
- **THEN** the outbound HTTP request to Forage includes header `Cache-Control: no-cache`

#### Scenario: Client does not request cache bypass
- **WHEN** an MCP client calls `forage_search` without `no_cache` or with `no_cache: false`
- **THEN** the outbound HTTP request to Forage does not include a `Cache-Control` header set by this tool

### Requirement: Search parameter validation
The system SHALL validate search parameters before forwarding them to Forage and return a clear error when validation fails.

#### Scenario: Query is empty or too long
- **WHEN** an MCP client calls `forage_search` with an empty `query` or a `query` longer than 500 characters
- **THEN** the system returns an error message identifying the invalid `query` and does not call Forage

#### Scenario: Limit is out of range
- **WHEN** an MCP client calls `forage_search` with `limit` less than 1 or greater than 50
- **THEN** the system returns an error message identifying the invalid `limit` and does not call Forage

#### Scenario: Engine value is not recognized
- **WHEN** an MCP client calls `forage_search` with an `engines` array containing a value outside `{google, bing, yahoo, duckduckgo, brave, ddg}`
- **THEN** the system returns an error message identifying the invalid engine name and does not call Forage

#### Scenario: ddg alias is accepted for DuckDuckGo
- **WHEN** an MCP client calls `forage_search` with `engines: ["ddg"]`
- **THEN** the system forwards the request to Forage without rejecting the engine value

### Requirement: Search uses global timeout only
The system SHALL apply the globally configured timeout (from CLI flag or environment) to every `forage_search` request, since the search tool has no per-request timeout parameter.

#### Scenario: Global timeout is exceeded during search
- **WHEN** a `forage_search` request does not complete within the global timeout
- **THEN** the system returns an error indicating the request timed out, including the timeout duration in seconds

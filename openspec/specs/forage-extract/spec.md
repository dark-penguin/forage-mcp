# forage-extract Specification

## Purpose

Lets MCP clients extract clean, structured content from web pages through a Forage backend using its hybrid static and browser extraction pipeline.

## Requirements

### Requirement: Extract tool is exposed to MCP clients
The system SHALL expose an MCP tool named `forage_extract` that accepts one or more URLs and optional parameters, and returns extracted page content from the Forage backend.

#### Scenario: Client invokes extract with valid URLs
- **WHEN** an MCP client calls `forage_extract` with a `urls` array containing 1 to 20 strings, each starting with `http://` or `https://`
- **THEN** the system forwards the request to the Forage `/extract` endpoint and returns a result containing `success: true`, a `data` array of per-URL extraction objects, and a `cache_status` field

#### Scenario: Client invokes extract without optional parameters
- **WHEN** an MCP client calls `forage_extract` with only `urls` provided
- **THEN** the system uses Forage's configured defaults for `formats`, `only_main_content`, `force_render`, `wait_for`, and `engine`

### Requirement: Extract result shape
The system SHALL return extraction results in a stable JSON structure containing a top-level `success` boolean, a `data` array of per-URL objects with `url`, `title`, `content`, `raw_content`, and `method` fields, and a top-level `cache_status` string.

#### Scenario: Forage returns extracted content
- **WHEN** the Forage `/extract` endpoint responds successfully with one or more extracted pages
- **THEN** each object in `data` contains `url`, `title`, `content`, `raw_content`, and a `method` value from `{static, browser, browser+solver, pdf, docx}`

#### Scenario: Forage returns no extractable content for a URL
- **WHEN** the Forage `/extract` endpoint responds successfully but a given URL yields no usable content
- **THEN** the system still includes that URL in `data` with empty or null content fields and does not fail the entire request

### Requirement: Cache status is surfaced from response header
The system SHALL read the `X-Forage-Cache` HTTP response header on every extract request and expose its value (`hit`, `miss`, `bypass`, or `disabled`) as the top-level `cache_status` field in the tool result.

#### Scenario: Forage responds with a cache miss
- **WHEN** the Forage `/extract` response includes header `X-Forage-Cache: miss`
- **THEN** the returned tool result contains `cache_status: "miss"`

#### Scenario: Forage omits the cache header
- **WHEN** the Forage `/extract` response does not include an `X-Forage-Cache` header
- **THEN** the returned tool result omits `cache_status` or sets it to a clearly empty value, without failing the request

### Requirement: no_cache flag is forwarded as Cache-Control header
The system SHALL send a `Cache-Control: no-cache` HTTP request header to Forage when the `no_cache` parameter is `true`, and SHALL NOT send that header when `no_cache` is `false` or omitted.

#### Scenario: Client requests cache bypass on extract
- **WHEN** an MCP client calls `forage_extract` with `no_cache: true`
- **THEN** the outbound HTTP request to Forage includes header `Cache-Control: no-cache`

#### Scenario: Client does not request cache bypass on extract
- **WHEN** an MCP client calls `forage_extract` without `no_cache` or with `no_cache: false`
- **THEN** the outbound HTTP request to Forage does not include a `Cache-Control` header set by this tool

### Requirement: Per-request timeout overrides global timeout
The system SHALL use the per-request `timeout` parameter (in seconds, range 1 to 120) on `forage_extract` when provided, and SHALL fall back to the globally configured timeout when the parameter is omitted or invalid.

#### Scenario: Client provides a valid per-request timeout
- **WHEN** an MCP client calls `forage_extract` with `timeout: 45`
- **THEN** the outbound HTTP request to Forage uses a 45-second timeout regardless of the global timeout setting

#### Scenario: Client omits the per-request timeout
- **WHEN** an MCP client calls `forage_extract` without a `timeout` parameter
- **THEN** the outbound HTTP request to Forage uses the globally configured timeout

#### Scenario: Per-request timeout is out of range
- **WHEN** an MCP client calls `forage_extract` with `timeout` less than 1 or greater than 120
- **THEN** the system returns an error message identifying the invalid `timeout` and does not call Forage

#### Scenario: Per-request timeout is exceeded
- **WHEN** a `forage_extract` request does not complete within its effective timeout
- **THEN** the system returns an error indicating the request timed out, including the timeout duration in seconds

### Requirement: Extract parameter validation
The system SHALL validate extract parameters before forwarding them to Forage and return a clear error when validation fails.

#### Scenario: URLs array is empty or too large
- **WHEN** an MCP client calls `forage_extract` with a `urls` array that is empty or contains more than 20 entries
- **THEN** the system returns an error message identifying the invalid `urls` and does not call Forage

#### Scenario: URL has an unsupported scheme
- **WHEN** an MCP client calls `forage_extract` with a `urls` array containing a string that does not start with `http://` or `https://`
- **THEN** the system returns an error message identifying the invalid URL and does not call Forage

#### Scenario: Format value is not recognized
- **WHEN** an MCP client calls `forage_extract` with a `formats` array containing a value outside `{markdown, html, raw_html}`
- **THEN** the system returns an error message identifying the invalid format and does not call Forage

#### Scenario: Engine value is not recognized
- **WHEN** an MCP client calls `forage_extract` with `engine` set to a value other than `trafilatura` or `readability`
- **THEN** the system returns an error message identifying the invalid engine and does not call Forage

#### Scenario: wait_for is passed through without validation
- **WHEN** an MCP client calls `forage_extract` with a `wait_for` CSS selector string
- **THEN** the system forwards the value to Forage as-is without client-side validation of the selector syntax

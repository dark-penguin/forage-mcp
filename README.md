# forage-mcp

An MCP (Model Context Protocol) server that bridges AI agents to the [Forage](https://github.com/aldemaroc/forage) web search and content extraction service. Exposes two tools — `forage_search` and `forage_extract` — via stdio transport.

## Installation

### From source

```bash
git clone https://github.com/dark-penguin/forage-mcp.git
cd forage-mcp
make build
```

The binary is produced at `./bin/forage-mcp`.

### Requirements

- Go 1.25+ (for building from source)
- A reachable Forage instance

## Configuration

### Environment Variables

| Variable | Required | Description |
|---|---|---|
| `FORAGE_URL` | Yes* | Base URL of the Forage instance (e.g., `http://localhost:3672`) |
| `FORAGE_API_KEY` | No | Optional Bearer token for authenticated Forage instances |
| `LOG_LEVEL` | No | Log level: `debug`, `info`, `warn` (default: `info`) |

*Required unless overridden by the `-u/--forage-url` flag.

### CLI Flags

| Flag | Description | Default |
|---|---|---|
| `-u`, `--forage-url` | Override `FORAGE_URL` | (env var) |
| `-k`, `--forage-api-key` | Override `FORAGE_API_KEY` | (env var) |
| `-t`, `--timeout` | Default timeout for API calls in seconds | `120` |
| `-v`, `--verbose` | Set log level to DEBUG | off |
| `-q`, `--quiet` | Set log level to WARN | off |

CLI flags take precedence over environment variables.

## Usage

### Claude Desktop

Add to your Claude Desktop MCP configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "forage": {
      "command": "/path/to/forage-mcp",
      "env": {
        "FORAGE_URL": "http://localhost:3672"
      }
    }
  }
}
```

### Agent Zero

Add to your Agent Zero MCP server configuration:

```json
{
  "forage": {
    "command": "/path/to/forage-mcp",
    "args": [],
    "env": {
      "FORAGE_URL": "http://localhost:3672"
    }
  }
}
```

### Direct CLI

```bash
# Using environment variables
export FORAGE_URL=http://localhost:3672
./bin/forage-mcp

# Using flags
./bin/forage-mcp -u http://localhost:3672 -t 60 -v
```

## Tools

### `forage_search`

Search the web using Forage's configured search provider.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | Yes | Search query (1–500 chars) |
| `limit` | integer | No | Number of results (1–50). Default: 5 |
| `language` | string | No | Language code (e.g., `pt-BR`, `en`) |
| `engines` | array[string] | No | Search engines: `google`, `bing`, `yahoo`, `duckduckgo`, `brave`, `ddg` |
| `no_cache` | boolean | No | Skip Forage's in-memory cache. Default: false |

**Example result:**

```json
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
  "cache_status": "hit"
}
```

### `forage_extract`

Extract clean content from web pages using Forage's hybrid extraction pipeline.

**Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `urls` | array[string] | Yes | URLs to extract (1–20, must start with `http://` or `https://`) |
| `formats` | array[string] | No | Output formats: `markdown`, `html`, `raw_html` |
| `only_main_content` | boolean | No | Extract only main content. Default: true |
| `force_render` | boolean | No | Force browser-based rendering. Default: false |
| `wait_for` | string | No | CSS selector to wait for before extraction |
| `timeout` | integer | No | Per-request timeout in seconds (1–120). Overrides global timeout |
| `engine` | string | No | Extraction engine: `trafilatura` or `readability` |
| `no_cache` | boolean | No | Skip Forage's in-memory cache. Default: false |

**Example result:**

```json
{
  "success": true,
  "data": [
    {
      "url": "https://example.com",
      "title": "Page Title",
      "content": "Clean markdown content...",
      "raw_content": "Raw extracted content",
      "method": "static"
    }
  ],
  "cache_status": "miss"
}
```

## Development

```bash
# Build
make build

# Run tests
make test

# Clean build artifacts
make clean
```

## License

See [LICENSE](LICENSE) for details.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var validExtractFormats = map[string]bool{
	"markdown": true,
	"html":     true,
	"raw_html": true,
}

var validExtractEngines = map[string]bool{
	"trafilatura": true,
	"readability": true,
}

// extractToolArgs represents the input parameters for the forage_extract tool.
type extractToolArgs struct {
	URLs            []string `json:"urls" jsonschema:"URLs to extract (1-20, must start with http:// or https://)"`
	Formats         []string `json:"formats,omitempty" jsonschema:"Output formats: markdown, html, raw_html"`
	OnlyMainContent *bool    `json:"only_main_content,omitempty" jsonschema:"Extract only main content, stripping nav/footer. Default: true"`
	ForceRender     *bool    `json:"force_render,omitempty" jsonschema:"Force browser-based rendering. Default: false"`
	WaitFor         string   `json:"wait_for,omitempty" jsonschema:"CSS selector to wait for before extraction (browser mode)"`
	Timeout         *int     `json:"timeout,omitempty" jsonschema:"Per-request timeout in seconds (1-120). Overrides global timeout"`
	Engine          string   `json:"engine,omitempty" jsonschema:"Extraction engine: trafilatura or readability"`
	NoCache         bool     `json:"no_cache,omitempty" jsonschema:"Skip Forage's in-memory cache"`
}

// extractToolResult is the structured output for the forage_extract tool.
type extractToolResult struct {
	Success     bool          `json:"success" jsonschema:"Whether the extraction succeeded"`
	Data        []ExtractItem `json:"data" jsonschema:"Extraction results per URL"`
	CacheStatus string        `json:"cache_status,omitempty" jsonschema:"Cache status from Forage: hit, miss, bypass, disabled"`
}

// validateExtractArgs validates the forage_extract tool parameters.
func validateExtractArgs(args *extractToolArgs) error {
	if len(args.URLs) == 0 {
		return fmt.Errorf("urls is required and must contain at least one URL")
	}
	if len(args.URLs) > 20 {
		return fmt.Errorf("urls must contain at most 20 entries (got %d)", len(args.URLs))
	}
	for i, url := range args.URLs {
		if url == "" {
			return fmt.Errorf("urls[%d] must be a non-empty string", i)
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("urls[%d] %q must start with http:// or https://", i, url)
		}
	}
	for _, format := range args.Formats {
		if !validExtractFormats[format] {
			return fmt.Errorf("invalid format %q: must be one of markdown, html, raw_html", format)
		}
	}
	if args.Timeout != nil {
		if *args.Timeout < 1 || *args.Timeout > 120 {
			return fmt.Errorf("timeout must be between 1 and 120 seconds (got %d)", *args.Timeout)
		}
	}
	if args.Engine != "" && !validExtractEngines[args.Engine] {
		return fmt.Errorf("invalid engine %q: must be trafilatura or readability", args.Engine)
	}
	return nil
}

// forageExtractHandler is the MCP tool handler for forage_extract.
func forageExtractHandler(ctx context.Context, req *mcp.CallToolRequest, input extractToolArgs) (*mcp.CallToolResult, extractToolResult, error) {
	slog.Info("incoming request", "tool", "forage_extract", "urls", len(input.URLs))

	if err := validateExtractArgs(&input); err != nil {
		return nil, extractToolResult{}, err
	}

	// Resolve effective timeout: per-request if valid, else global
	effectiveTimeout := serverConfig.TimeoutSeconds
	if input.Timeout != nil {
		effectiveTimeout = *input.Timeout
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(effectiveTimeout)*time.Second)
	defer cancel()

	extractParams := ExtractParams{
		URLs:            input.URLs,
		Formats:         input.Formats,
		OnlyMainContent: input.OnlyMainContent,
		ForceRender:     input.ForceRender,
		WaitFor:         input.WaitFor,
		Timeout:         input.Timeout,
		Engine:          input.Engine,
	}

	result, err := forageClient.Extract(ctx, extractParams, input.NoCache)
	if err != nil {
		return nil, extractToolResult{}, err
	}

	toolResult := extractToolResult{
		Success:     result.Success,
		Data:        result.Data,
		CacheStatus: result.Cache,
	}

	// Ensure data is an empty array, not null
	if toolResult.Data == nil {
		toolResult.Data = []ExtractItem{}
	}

	slog.Info("extract complete", "urls", len(toolResult.Data), "cache", toolResult.CacheStatus)

	return nil, toolResult, nil
}

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var validSearchEngines = map[string]bool{
	"google":     true,
	"bing":       true,
	"yahoo":      true,
	"duckduckgo": true,
	"brave":      true,
	"ddg":        true, // alias for duckduckgo
}

// searchToolArgs represents the input parameters for the forage_search tool.
type searchToolArgs struct {
	Query    string   `json:"query" jsonschema:"Search query (1-500 characters)"`
	Limit    *int     `json:"limit,omitempty" jsonschema:"Number of results (1-50). Default: 5"`
	Language string   `json:"language,omitempty" jsonschema:"Language code (e.g., pt-BR, en)"`
	Engines  []string `json:"engines,omitempty" jsonschema:"Search engines to use: google, bing, yahoo, duckduckgo, brave, ddg"`
	NoCache  bool     `json:"no_cache,omitempty" jsonschema:"Skip Forage's in-memory cache"`
}

// searchToolResult is the structured output for the forage_search tool.
type searchToolResult struct {
	Success     bool        `json:"success" jsonschema:"Whether the search succeeded"`
	Data        SearchData  `json:"data" jsonschema:"Search results"`
	CacheStatus string      `json:"cache_status,omitempty" jsonschema:"Cache status from Forage: hit, miss, bypass, disabled"`
}

// validateSearchArgs validates the forage_search tool parameters.
func validateSearchArgs(args *searchToolArgs) error {
	if args.Query == "" {
		return fmt.Errorf("query is required and must be a non-empty string")
	}
	if len(args.Query) > 500 {
		return fmt.Errorf("query must be at most 500 characters (got %d)", len(args.Query))
	}
	if args.Limit != nil {
		if *args.Limit < 1 || *args.Limit > 50 {
			return fmt.Errorf("limit must be between 1 and 50 (got %d)", *args.Limit)
		}
	}
	for _, engine := range args.Engines {
		if !validSearchEngines[engine] {
			return fmt.Errorf("invalid engine %q: must be one of google, bing, yahoo, duckduckgo, brave, ddg", engine)
		}
	}
	return nil
}

// searchHandler is the MCP tool handler for forage_search.
func (a *app) searchHandler(ctx context.Context, req *mcp.CallToolRequest, input searchToolArgs) (*mcp.CallToolResult, searchToolResult, error) {
	slog.Info("incoming request", "tool", "forage_search", "query", input.Query)

	if err := validateSearchArgs(&input); err != nil {
		return nil, searchToolResult{}, err
	}

	searchParams := SearchParams{
		Query:    input.Query,
		Limit:    input.Limit,
		Language: input.Language,
		Engines:  input.Engines,
	}

	result, err := a.client.Search(ctx, searchParams, input.NoCache)
	if err != nil {
		return nil, searchToolResult{}, err
	}

	toolResult := searchToolResult{
		Success:     result.Success,
		Data:        result.Data,
		CacheStatus: result.Cache,
	}

	// Ensure web results is an empty array, not null
	if toolResult.Data.Web == nil {
		toolResult.Data.Web = []WebResult{}
	}

	slog.Info("search complete", "results", len(toolResult.Data.Web), "cache", toolResult.CacheStatus)

	return nil, toolResult, nil
}

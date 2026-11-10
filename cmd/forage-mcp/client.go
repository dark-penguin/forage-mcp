package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// SearchParams holds the parameters for a search request to Forage.
type SearchParams struct {
	Query    string   `json:"query"`
	Limit    *int     `json:"limit,omitempty"`
	Language string   `json:"language,omitempty"`
	Engines  []string `json:"engines,omitempty"`
}

// ExtractParams holds the parameters for an extract request to Forage.
type ExtractParams struct {
	URLs             []string `json:"urls"`
	Formats          []string `json:"formats,omitempty"`
	OnlyMainContent  *bool    `json:"only_main_content,omitempty"`
	ForceRender      *bool    `json:"force_render,omitempty"`
	WaitFor          string   `json:"wait_for,omitempty"`
	Timeout          *int     `json:"timeout,omitempty"`
	Engine           string   `json:"engine,omitempty"`
}

// WebResult represents a single web search result.
type WebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

// SearchResult is the response from Forage's /search endpoint.
type SearchResult struct {
	Success bool        `json:"success"`
	Data    SearchData  `json:"data"`
	Cache   string      `json:"-"` // populated from X-Forage-Cache header
}

type SearchData struct {
	Web []WebResult `json:"web"`
}

// ExtractItem represents a single URL extraction result.
type ExtractItem struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	RawContent string `json:"raw_content"`
	Method     string `json:"method"`
}

// ExtractResult is the response from Forage's /extract endpoint.
type ExtractResult struct {
	Success bool          `json:"success"`
	Data    []ExtractItem `json:"data"`
	Cache   string        `json:"-"` // populated from X-Forage-Cache header
}

// ForageClient wraps an HTTP client for communicating with the Forage API.
type ForageClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewForageClient creates a new ForageClient with the given base URL, optional API key, and timeout.
func NewForageClient(baseURL, apiKey string, timeoutSeconds int) *ForageClient {
	return &ForageClient{
		httpClient: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

// doRequest performs an HTTP request to the Forage API, setting standard headers,
// reading the X-Forage-Cache response header, and decoding the JSON body.
func (c *ForageClient) doRequest(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) (json.RawMessage, string, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	slog.Debug("sending forage request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
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
	defer resp.Body.Close()

	cacheStatus := resp.Header.Get("X-Forage-Cache")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("forage returned error", "status", resp.StatusCode, "body", string(respBody))
		return nil, cacheStatus, fmt.Errorf("forage request failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("forage response received", "status", resp.StatusCode, "cache", cacheStatus)

	return json.RawMessage(respBody), cacheStatus, nil
}

// Search performs a web search via Forage's /search endpoint.
func (c *ForageClient) Search(ctx context.Context, params SearchParams, noCache bool) (*SearchResult, error) {
	extraHeaders := make(map[string]string)
	if noCache {
		extraHeaders["Cache-Control"] = "no-cache"
	}

	raw, cacheStatus, err := c.doRequest(ctx, http.MethodPost, "/search", params, extraHeaders)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Error("failed to decode search response", "error", err)
		return nil, fmt.Errorf("failed to decode forage search response: %w", err)
	}
	result.Cache = cacheStatus
	return &result, nil
}

// Extract extracts content from URLs via Forage's /extract endpoint.
func (c *ForageClient) Extract(ctx context.Context, params ExtractParams, noCache bool) (*ExtractResult, error) {
	extraHeaders := make(map[string]string)
	if noCache {
		extraHeaders["Cache-Control"] = "no-cache"
	}

	raw, cacheStatus, err := c.doRequest(ctx, http.MethodPost, "/extract", params, extraHeaders)
	if err != nil {
		return nil, err
	}

	var result ExtractResult
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Error("failed to decode extract response", "error", err)
		return nil, fmt.Errorf("failed to decode forage extract response: %w", err)
	}
	result.Cache = cacheStatus
	return &result, nil
}

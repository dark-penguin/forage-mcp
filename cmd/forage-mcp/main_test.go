package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEndToEnd(t *testing.T) {
	// Set up a mock Forage server
	forageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("X-Forage-Cache", "miss")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"web": []map[string]interface{}{
						{"title": "Go MCP SDK", "url": "https://github.com/modelcontextprotocol/go-sdk", "description": "Official Go SDK for MCP", "position": 1},
					},
				},
			})
		case "/extract":
			w.Header().Set("X-Forage-Cache", "hit")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": []map[string]interface{}{
					{"url": "https://example.com", "title": "Example", "content": "Example content", "raw_content": "<html>...</html>", "method": "static"},
				},
			})
		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		}
	}))
	defer forageServer.Close()

	a := &app{
		cfg:    &Config{ForageURL: forageServer.URL, TimeoutSeconds: 10},
		client: NewForageClient(forageServer.URL, "", 10),
	}

	// Create MCP server with tools
	server := mcp.NewServer(
		&mcp.Implementation{Name: "forage-mcp", Version: "v1.0.0"},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "forage_search",
		Description: "Search the web using Forage",
	}, a.searchHandler)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "forage_extract",
		Description: "Extract content from URLs using Forage",
	}, a.extractHandler)

	ctx := context.Background()

	// Use in-memory transport for testing
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Connect server and client
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	defer clientSession.Close()

	// Test initialize (happens automatically during Connect)
	initResult := clientSession.InitializeResult()
	if initResult == nil {
		t.Fatal("expected non-nil InitializeResult")
	}
	if initResult.ServerInfo.Name != "forage-mcp" {
		t.Errorf("server name = %q, want forage-mcp", initResult.ServerInfo.Name)
	}

	// Test tools/list
	toolsResult, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(toolsResult.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(toolsResult.Tools))
	}

	toolNames := map[string]bool{}
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["forage_search"] {
		t.Error("forage_search tool not found in tools list")
	}
	if !toolNames["forage_extract"] {
		t.Error("forage_extract tool not found in tools list")
	}

	// Test forage_search tool call
	searchResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "forage_search",
		Arguments: map[string]interface{}{
			"query": "golang mcp",
		},
	})
	if err != nil {
		t.Fatalf("CallTool forage_search failed: %v", err)
	}
	if searchResult.IsError {
		t.Errorf("forage_search returned error: %v", searchResult.Content)
	}

	// Verify the result contains success and data.web
	if searchResult.StructuredContent != nil {
		var searchResp map[string]interface{}
		b, err := json.Marshal(searchResult.StructuredContent)
		if err != nil {
			t.Fatalf("failed to marshal structured content: %v", err)
		}
		if err := json.Unmarshal(b, &searchResp); err != nil {
			t.Fatalf("failed to unmarshal search result: %v", err)
		}
		if success, ok := searchResp["success"].(bool); !ok || !success {
			t.Error("expected success: true")
		}
		data, ok := searchResp["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data object")
		}
		web, ok := data["web"].([]interface{})
		if !ok || len(web) == 0 {
			t.Error("expected non-empty data.web array")
		}
	}

	// Test forage_extract tool call
	extractResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "forage_extract",
		Arguments: map[string]interface{}{
			"urls": []string{"https://example.com"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool forage_extract failed: %v", err)
	}
	if extractResult.IsError {
		t.Errorf("forage_extract returned error: %v", extractResult.Content)
	}

	// Test validation error - empty query
	errResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "forage_search",
		Arguments: map[string]interface{}{
			"query": "",
		},
	})
	if err != nil {
		t.Fatalf("CallTool forage_search (empty query) failed: %v", err)
	}
	if !errResult.IsError {
		t.Error("expected isError: true for empty query")
	}
}

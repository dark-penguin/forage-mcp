package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type app struct {
	cfg    *Config
	client *ForageClient
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	slog.SetDefault(slog.New(handler))

	a := &app{
		cfg:    cfg,
		client: NewForageClient(cfg.ForageURL, cfg.APIKey, cfg.TimeoutSeconds),
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "forage-mcp",
			Version: "v1.0.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "forage_search",
		Description: "Search the web using Forage's configured search provider (Google, Bing, Yahoo, DuckDuckGo, Brave). Returns structured results with title, URL, description, and position.",
	}, a.searchHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "forage_extract",
		Description: "Extract clean content from web pages using Forage's hybrid static + browser extraction pipeline. Returns markdown, HTML, or raw content per URL.",
	}, a.extractHandler)

	slog.Info("starting forage-mcp server", "forage_url", cfg.ForageURL, "timeout", cfg.TimeoutSeconds)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

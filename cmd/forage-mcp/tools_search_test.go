package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func intPtr(i int) *int { return &i }

func TestValidateSearchArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    searchToolArgs
		wantErr bool
		errSub  string
	}{
		{name: "valid query", args: searchToolArgs{Query: "golang mcp"}, wantErr: false},
		{name: "empty query returns error", args: searchToolArgs{Query: ""}, wantErr: true, errSub: "query is required"},
		{name: "501 char query returns error", args: searchToolArgs{Query: strings.Repeat("a", 501)}, wantErr: true, errSub: "at most 500"},
		{name: "500 char query is valid", args: searchToolArgs{Query: strings.Repeat("a", 500)}, wantErr: false},
		{name: "limit=0 returns error", args: searchToolArgs{Query: "test", Limit: intPtr(0)}, wantErr: true, errSub: "between 1 and 50"},
		{name: "limit=51 returns error", args: searchToolArgs{Query: "test", Limit: intPtr(51)}, wantErr: true, errSub: "between 1 and 50"},
		{name: "limit=1 is valid", args: searchToolArgs{Query: "test", Limit: intPtr(1)}, wantErr: false},
		{name: "limit=50 is valid", args: searchToolArgs{Query: "test", Limit: intPtr(50)}, wantErr: false},
		{name: "invalid engine returns error", args: searchToolArgs{Query: "test", Engines: []string{"yandex"}}, wantErr: true, errSub: "invalid engine"},
		{name: "ddg alias is accepted", args: searchToolArgs{Query: "test", Engines: []string{"ddg"}}, wantErr: false},
		{name: "all valid engines", args: searchToolArgs{Query: "test", Engines: []string{"google", "bing", "yahoo", "duckduckgo", "brave"}}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchArgs(&tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSub)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestForageSearchHandler(t *testing.T) {
	var gotCacheCtrl string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCacheCtrl = r.Header.Get("Cache-Control")
		w.Header().Set("X-Forage-Cache", "miss")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"web": []map[string]interface{}{
					{"title": "Result 1", "url": "https://example.com/1", "description": "First result", "position": 1},
				},
			},
		})
	}))
	defer server.Close()

	origClient := forageClient
	origConfig := serverConfig
	forageClient = NewForageClient(server.URL, "", 10)
	serverConfig = &Config{ForageURL: server.URL, TimeoutSeconds: 10}
	defer func() {
		forageClient = origClient
		serverConfig = origConfig
	}()

	tests := []struct {
		name     string
		argsJSON string
		wantErr  bool
		errSub   string
	}{
		{name: "valid query returns results", argsJSON: `{"query":"golang mcp"}`, wantErr: false},
		{name: "empty query returns error", argsJSON: `{"query":""}`, wantErr: true, errSub: "query is required"},
		{name: "501 char query returns error", argsJSON: `{"query":"` + strings.Repeat("a", 501) + `"}`, wantErr: true, errSub: "at most 500"},
		{name: "limit=0 returns error", argsJSON: `{"query":"test","limit":0}`, wantErr: true, errSub: "between 1 and 50"},
		{name: "limit=51 returns error", argsJSON: `{"query":"test","limit":51}`, wantErr: true, errSub: "between 1 and 50"},
		{name: "invalid engine returns error", argsJSON: `{"query":"test","engines":["yandex"]}`, wantErr: true, errSub: "invalid engine"},
		{name: "ddg alias is accepted", argsJSON: `{"query":"test","engines":["ddg"]}`, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCacheCtrl = ""
			var args searchToolArgs
			if err := json.Unmarshal([]byte(tt.argsJSON), &args); err != nil {
				t.Fatalf("failed to unmarshal test args: %v", err)
			}
			_, _, err := forageSearchHandler(context.Background(), nil, args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSub)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}

	t.Run("no_cache=true sends Cache-Control header", func(t *testing.T) {
		gotCacheCtrl = ""
		args := searchToolArgs{Query: "test", NoCache: true}
		_, _, err := forageSearchHandler(context.Background(), nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotCacheCtrl != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", gotCacheCtrl)
		}
	})

	t.Run("no_cache=false omits Cache-Control header", func(t *testing.T) {
		gotCacheCtrl = ""
		args := searchToolArgs{Query: "test", NoCache: false}
		_, _, err := forageSearchHandler(context.Background(), nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotCacheCtrl != "" {
			t.Errorf("Cache-Control = %q, want empty", gotCacheCtrl)
		}
	})

	t.Run("Forage 500 returns error with status code", func(t *testing.T) {
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"boom"}`))
		}))
		defer errServer.Close()

		forageClient = NewForageClient(errServer.URL, "", 10)
		args := searchToolArgs{Query: "test"}
		_, _, err := forageSearchHandler(context.Background(), nil, args)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error should contain HTTP status code: %v", err)
		}
	})
}

package main

import (
	"time"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForageClientDoRequest(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		cacheHeader   string
		apiKey        string
		noCache       bool
		wantErr       bool
		wantCache     string
		wantAuthHdr   string
		wantCacheCtrl string
	}{
		{
			name:      "successful 200 response",
			status:    http.StatusOK,
			body:      `{"success":true}`,
			cacheHeader: "hit",
			wantErr:   false,
			wantCache: "hit",
		},
		{
			name:      "non-2xx error response",
			status:    http.StatusInternalServerError,
			body:      `{"error":"internal server error"}`,
			wantErr:   true,
		},
		{
			name:      "missing X-Forage-Cache header",
			status:    http.StatusOK,
			body:      `{"success":true}`,
			cacheHeader: "",
			wantErr:   false,
			wantCache: "",
		},
		{
			name:        "API key present in Authorization header",
			status:      http.StatusOK,
			body:        `{"success":true}`,
			apiKey:      "test-token",
			wantErr:     false,
			wantAuthHdr: "Bearer test-token",
		},
		{
			name:          "Cache-Control header forwarded when no_cache",
			status:        http.StatusOK,
			body:          `{"success":true}`,
			noCache:       true,
			wantErr:       false,
			wantCacheCtrl: "no-cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			var gotCacheCtrl string
			var gotContentType string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				gotCacheCtrl = r.Header.Get("Cache-Control")
				gotContentType = r.Header.Get("Content-Type")

				if tt.cacheHeader != "" {
					w.Header().Set("X-Forage-Cache", tt.cacheHeader)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewForageClient(server.URL, tt.apiKey, 10)

			extraHeaders := make(map[string]string)
			if tt.noCache {
				extraHeaders["Cache-Control"] = "no-cache"
			}

			raw, cacheStatus, err := client.doRequest(context.Background(), http.MethodPost, "/search", map[string]string{"query": "test"}, extraHeaders)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), http.StatusText(tt.status)) && !strings.Contains(err.Error(), "HTTP") {
					t.Errorf("error should mention HTTP status: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cacheStatus != tt.wantCache {
				t.Errorf("cache status = %q, want %q", cacheStatus, tt.wantCache)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if tt.wantAuthHdr != "" && gotAuth != tt.wantAuthHdr {
				t.Errorf("Authorization = %q, want %q", gotAuth, tt.wantAuthHdr)
			}
			if tt.wantCacheCtrl != "" && gotCacheCtrl != tt.wantCacheCtrl {
				t.Errorf("Cache-Control = %q, want %q", gotCacheCtrl, tt.wantCacheCtrl)
			}
			// Verify raw is valid JSON
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Errorf("raw response is not valid JSON: %v", err)
			}
		})
	}
}

func TestForageClientSearch(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		cacheHdr   string
		wantErr    bool
		wantWebLen int
	}{
		{
			name: "valid response mapping",
			status: http.StatusOK,
			body: `{"success":true,"data":{"web":[{"title":"Test","url":"https://example.com","description":"A test","position":1}]}}`,
			cacheHdr: "miss",
			wantErr:  false,
			wantWebLen: 1,
		},
		{
			name:       "empty results",
			status:     http.StatusOK,
			body:       `{"success":true,"data":{"web":[]}}`,
			cacheHdr:   "hit",
			wantErr:    false,
			wantWebLen: 0,
		},
		{
			name:    "Forage 500 error",
			status:  http.StatusInternalServerError,
			body:    `{"error":"boom"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.cacheHdr != "" {
					w.Header().Set("X-Forage-Cache", tt.cacheHdr)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewForageClient(server.URL, "", 10)
			result, err := client.Search(context.Background(), SearchParams{Query: "test"}, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "500") {
					t.Errorf("error should contain HTTP status code: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Data.Web) != tt.wantWebLen {
				t.Errorf("web results length = %d, want %d", len(result.Data.Web), tt.wantWebLen)
			}
			if result.Cache != tt.cacheHdr {
				t.Errorf("cache = %q, want %q", result.Cache, tt.cacheHdr)
			}
		})
	}
}

func TestForageClientExtract(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		cacheHdr   string
		wantErr    bool
		wantDataLen int
	}{
		{
			name: "valid response mapping",
			status: http.StatusOK,
			body: `{"success":true,"data":[{"url":"https://example.com","title":"Test Page","content":"Hello world","raw_content":"<html>...</html>","method":"static"}]}`,
			cacheHdr: "miss",
			wantErr:  false,
			wantDataLen: 1,
		},
		{
			name:    "Forage 400 error",
			status:  http.StatusBadRequest,
			body:    `{"error":"invalid url"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.cacheHdr != "" {
					w.Header().Set("X-Forage-Cache", tt.cacheHdr)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewForageClient(server.URL, "", 10)
			result, err := client.Extract(context.Background(), ExtractParams{URLs: []string{"https://example.com"}}, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "400") {
					t.Errorf("error should contain HTTP status code: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Data) != tt.wantDataLen {
				t.Errorf("data length = %d, want %d", len(result.Data), tt.wantDataLen)
			}
		})
	}
}

func TestForageClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context timeout so the request is cancelled
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	client := NewForageClient(server.URL, "", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, SearchParams{Query: "test"}, false)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestForageClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client's 50ms timeout
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("X-Forage-Cache", "miss")
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Construct client directly with a 50ms timeout (NewForageClient only accepts whole seconds)
	client := &ForageClient{
		httpClient: &http.Client{Timeout: 50 * time.Millisecond},
		baseURL:    server.URL,
	}

	_, _, err := client.doRequest(context.Background(), http.MethodPost, "/search", map[string]string{"query": "test"}, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should contain \"timed out\", got: %v", err)
	}
}

func TestForageClientLargeResponseBody(t *testing.T) {
	const cap = 50 << 20 // 50 MB, matches the LimitReader in doRequest

	// Payload just over the 50MB cap to confirm truncation
	overLimit := make([]byte, cap+1024)
	for i := range overLimit {
		overLimit[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Forage-Cache", "miss")
		w.Write(overLimit)
	}))
	defer server.Close()

	client := NewForageClient(server.URL, "", 10)

	raw, _, err := client.doRequest(context.Background(), http.MethodPost, "/search", map[string]string{"query": "test"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LimitReader caps the body at exactly 50MB
	if len(raw) != cap {
		t.Errorf("response body length = %d, want %d (50MB cap)", len(raw), cap)
	}
}

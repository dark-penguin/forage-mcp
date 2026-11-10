package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateExtractArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    extractToolArgs
		wantErr bool
		errSub  string
	}{
		{
			name:    "valid URLs",
			args:    extractToolArgs{URLs: []string{"https://example.com"}},
			wantErr: false,
		},
		{
			name:    "empty urls array returns error",
			args:    extractToolArgs{URLs: []string{}},
			wantErr: true,
		errSub:  "at least one URL",
		},
		{
			name:    "21 URLs returns error",
			args:    extractToolArgs{URLs: make([]string, 21)},
			wantErr: true,
		errSub:  "at most 20",
		},
		{
			name:    "20 URLs is valid",
			args:    extractToolArgs{URLs: []string{"https://example.com/1", "https://example.com/2", "https://example.com/3", "https://example.com/4", "https://example.com/5", "https://example.com/6", "https://example.com/7", "https://example.com/8", "https://example.com/9", "https://example.com/10", "https://example.com/11", "https://example.com/12", "https://example.com/13", "https://example.com/14", "https://example.com/15", "https://example.com/16", "https://example.com/17", "https://example.com/18", "https://example.com/19", "https://example.com/20"}},
			wantErr: false,
		},
		{
			name:    "ftp scheme returns error",
			args:    extractToolArgs{URLs: []string{"ftp://example.com"}},
			wantErr: true,
		errSub:  "http:// or https://",
		},
		{
			name:    "empty URL string returns error",
			args:    extractToolArgs{URLs: []string{""}},
			wantErr: true,
		errSub:  "non-empty",
		},
		{
			name:    "invalid format returns error",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Formats: []string{"text"}},
			wantErr: true,
		errSub:  "invalid format",
		},
		{
			name:    "valid formats",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Formats: []string{"markdown", "html", "raw_html"}},
			wantErr: false,
		},
		{
			name:    "timeout=0 returns error",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Timeout: intPtr(0)},
			wantErr: true,
		errSub:  "between 1 and 120",
		},
		{
			name:    "timeout=121 returns error",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Timeout: intPtr(121)},
			wantErr: true,
		errSub:  "between 1 and 120",
		},
		{
			name:    "timeout=1 is valid",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Timeout: intPtr(1)},
			wantErr: false,
		},
		{
			name:    "timeout=120 is valid",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Timeout: intPtr(120)},
			wantErr: false,
		},
		{
			name:    "invalid engine returns error",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Engine: "lynx"},
			wantErr: true,
		errSub:  "trafilatura or readability",
		},
		{
			name:    "valid engine trafilatura",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Engine: "trafilatura"},
			wantErr: false,
		},
		{
			name:    "valid engine readability",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, Engine: "readability"},
			wantErr: false,
		},
		{
			name:    "wait_for is passed through without validation",
			args:    extractToolArgs{URLs: []string{"https://example.com"}, WaitFor: ".invalid-css-[[["},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExtractArgs(&tt.args)
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

func TestForageExtractHandler(t *testing.T) {
	var gotCacheCtrl string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCacheCtrl = r.Header.Get("Cache-Control")
		w.Header().Set("X-Forage-Cache", "miss")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": []map[string]interface{}{
				{"url": "https://example.com", "title": "Test Page", "content": "Hello", "raw_content": "<html></html>", "method": "static"},
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
		{
			name:     "valid URLs return results",
			argsJSON: `{"urls":["https://example.com"]}`,
			wantErr:  false,
		},
		{
			name:     "empty urls array returns error",
			argsJSON: `{"urls":[]}`,
			wantErr:  true,
		errSub:   "at least one URL",
		},
		{
			name:     "21 URLs returns error",
			argsJSON: `{"urls":[` + strings.Repeat(`"https://example.com",`, 20) + `"https://example.com"]}`,
			wantErr:  true,
		errSub:   "at most 20",
		},
		{
			name:     "ftp scheme returns error",
			argsJSON: `{"urls":["ftp://example.com"]}`,
			wantErr:  true,
		errSub:   "http:// or https://",
		},
		{
			name:     "invalid format returns error",
			argsJSON: `{"urls":["https://example.com"],"formats":["text"]}`,
			wantErr:  true,
		errSub:   "invalid format",
		},
		{
			name:     "timeout=0 returns error",
			argsJSON: `{"urls":["https://example.com"],"timeout":0}`,
			wantErr:  true,
		errSub:   "between 1 and 120",
		},
		{
			name:     "timeout=121 returns error",
			argsJSON: `{"urls":["https://example.com"],"timeout":121}`,
			wantErr:  true,
		errSub:   "between 1 and 120",
		},
		{
			name:     "invalid engine returns error",
			argsJSON: `{"urls":["https://example.com"],"engine":"lynx"}`,
			wantErr:  true,
		errSub:   "trafilatura or readability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCacheCtrl = ""
			var args extractToolArgs
			if err := json.Unmarshal([]byte(tt.argsJSON), &args); err != nil {
				t.Fatalf("failed to unmarshal test args: %v", err)
			}

			_, _, err := forageExtractHandler(context.Background(), nil, args)

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

	// Test no_cache header forwarding
	t.Run("no_cache=true sends Cache-Control header", func(t *testing.T) {
		gotCacheCtrl = ""
		args := extractToolArgs{URLs: []string{"https://example.com"}, NoCache: true}
		_, _, err := forageExtractHandler(context.Background(), nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotCacheCtrl != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", gotCacheCtrl)
		}
	})

	// Test Forage 400 error
	t.Run("Forage 400 returns error with status code", func(t *testing.T) {
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid url"}`))
		}))
		defer errServer.Close()

		forageClient = NewForageClient(errServer.URL, "", 10)
		args := extractToolArgs{URLs: []string{"https://example.com"}}
		_, _, err := forageExtractHandler(context.Background(), nil, args)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error should contain HTTP status code: %v", err)
		}
	})
}

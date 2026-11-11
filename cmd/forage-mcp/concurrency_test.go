package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConcurrentHandlerCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Forage-Cache", "miss")
		switch r.URL.Path {
		case "/search":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"web": []map[string]interface{}{
						{"title": "Result", "url": "https://example.com", "description": "desc", "position": 1},
					},
				},
			})
		case "/extract":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": []map[string]interface{}{
					{"url": "https://example.com", "title": "Page", "content": "content", "raw_content": "<html></html>", "method": "static"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	a := &app{
		cfg:    &Config{ForageURL: server.URL, TimeoutSeconds: 10},
		client: NewForageClient(server.URL, "", 10),
	}

	const goroutines = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*2)

	for i := 0; i < goroutines; i++ {
		wg.Add(2)

		// Concurrent search handler calls
		go func() {
			defer wg.Done()
			_, _, err := a.searchHandler(context.Background(), nil, searchToolArgs{Query: "concurrent test"})
			if err != nil {
				errCh <- err
			}
		}()

		// Concurrent extract handler calls
		go func() {
			defer wg.Done()
			_, _, err := a.extractHandler(context.Background(), nil, extractToolArgs{URLs: []string{"https://example.com"}})
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent handler call failed: %v", err)
	}
}

package distcache

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPPool_ServeHTTP(t *testing.T) {
	// Create a test cache group
	NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			m := map[string]string{
				"Tom":  "630",
				"Jack": "589",
				"Sam":  "567",
			}
			if v, ok := m[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	// Create HTTP pool
	pool := NewHTTPPool("test-server")

	// Test case: Access a valid cache group and key
	t.Run("Get_Existing_Key", func(t *testing.T) {
		// Create test request
		req := httptest.NewRequest("GET", "/_distcache/scores/Tom", nil)
		w := httptest.NewRecorder()

		// Execute request
		pool.ServeHTTP(w, req)

		// Check response
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK; got %v", resp.StatusCode)
		}

		if string(body) != "630" {
			t.Errorf("Expected body '630'; got %s", body)
		}

		if resp.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("Expected Content-Type 'application/octet-stream'; got %s",
				resp.Header.Get("Content-Type"))
		}
	})

	// Test case: Access a non-existing key
	t.Run("Get_NonExisting_Key", func(t *testing.T) {
		// Create test request
		req := httptest.NewRequest("GET", "/_distcache/scores/Unknown", nil)
		w := httptest.NewRecorder()

		// Execute request
		pool.ServeHTTP(w, req)

		// Check response
		resp := w.Result()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status Internal Server Error; got %v", resp.StatusCode)
		}
	})

	// Test case: Access a non-existing cache group
	t.Run("Get_NonExisting_Group", func(t *testing.T) {
		// Create test request
		req := httptest.NewRequest("GET", "/_distcache/unknown-group/Tom", nil)
		w := httptest.NewRecorder()

		// Execute request
		pool.ServeHTTP(w, req)

		// Check response
		resp := w.Result()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status Not Found; got %v", resp.StatusCode)
		}
	})

	// Test case: Invalid path format
	t.Run("Invalid_Path_Format", func(t *testing.T) {
		// Create test request - missing key part
		req := httptest.NewRequest("GET", "/_distcache/scores", nil)
		w := httptest.NewRecorder()

		// Execute request
		pool.ServeHTTP(w, req)

		// Check response
		resp := w.Result()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request; got %v", resp.StatusCode)
		}
	})

	// Test case: Completely wrong path
	t.Run("Completely_Wrong_Path", func(t *testing.T) {
		// Create test request
		req := httptest.NewRequest("GET", "/wrong/path", nil)
		w := httptest.NewRecorder()

		// Expect a panic
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected code to panic for wrong path")
			}
		}()

		// Execute request
		pool.ServeHTTP(w, req)
	})
}

// Test HTTP client functionality (if you've implemented an HTTP client)
// This part needs to be customized based on your specific implementation
func TestHTTPClient(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/_distcache/") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		parts := strings.SplitN(r.URL.Path[len("/_distcache/"):], "/", 2)
		if len(parts) != 2 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		groupName := parts[0]
		key := parts[1]

		// Simulate a simple response
		if groupName == "scores" && key == "Tom" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte("630"))
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	// Here you can test your HTTP client code
	// For example:
	// client := NewHTTPClient(server.URL)
	// value, err := client.Get("scores", "Tom")
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if string(value) != "630" {
	// 	t.Errorf("Expected '630', got '%s'", value)
	// }
}

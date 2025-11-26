package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type GameResult struct {
	Player1 string `json:"player1"`
	Player2 string `json:"player2"`
	Winner  string `json:"winner"`
	Pattern string `json:"pattern"`
	IsTie   bool   `json:"isTie"`
}

// Mock handler for testing
func mockGameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var game GameResult
	if err := json.NewDecoder(r.Body).Decode(&game); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

func TestConcurrentGameSubmissions(t *testing.T) {
	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	handler := http.HandlerFunc(mockGameHandler)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			game := GameResult{
				Player1: "Alice",
				Player2: "Bob",
				Winner:  "Alice",
				Pattern: "row1",
				IsTie:   false,
			}
			body, _ := json.Marshal(game)
			req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				errors <- http.ErrNotSupported
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}
}

func TestAPIResponseTime(t *testing.T) {
	handler := http.HandlerFunc(mockGameHandler)
	game := GameResult{
		Player1: "Alice",
		Player2: "Bob",
		Winner:  "Alice",
		Pattern: "row1",
		IsTie:   false,
	}
	body, _ := json.Marshal(game)

	start := time.Now()
	req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("API response time %v exceeds 100ms threshold", duration)
	}
}

func TestHighLoadRequests(t *testing.T) {
	const iterations = 1000
	handler := http.HandlerFunc(mockGameHandler)

	for i := 0; i < iterations; i++ {
		game := GameResult{
			Player1: "Alice",
			Player2: "Bob",
			Winner:  "Alice",
			Pattern: "row1",
			IsTie:   false,
		}
		body, _ := json.Marshal(game)
		req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i, w.Code)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	handler := http.HandlerFunc(mockGameHandler)

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{"GET method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"Invalid JSON", http.MethodPost, "not json", http.StatusBadRequest},
		{"Empty body", http.MethodPost, "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/game", bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func resetMetrics() {
	gamesTotal.Reset()
	winsTotal.Reset()
	playerGamesTotal.Reset()
	tiesTotal.Reset()
	winStreakGauge.Reset()
	dynamoDBOps.Reset()
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()
	winStreaks = make(map[string]int)
}

func TestGameHandler_Win(t *testing.T) {
	resetMetrics()

	game := GameResult{
		Player1: "Alice",
		Player2: "Bob",
		Winner:  "Alice",
		Pattern: "row1",
		IsTie:   false,
		Mode:    "local",
	}
	body, _ := json.Marshal(game)

	req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gameHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify metrics
	if got := testutil.ToFloat64(gamesTotal.WithLabelValues("win", "local")); got != 1 {
		t.Errorf("expected games_total{result=win,mode=local} = 1, got %f", got)
	}
	if got := testutil.ToFloat64(winsTotal.WithLabelValues("Alice", "row1", "local")); got != 1 {
		t.Errorf("expected wins_total{player=Alice,pattern=row1,mode=local} = 1, got %f", got)
	}
	if got := testutil.ToFloat64(playerGamesTotal.WithLabelValues("Alice", "local")); got != 1 {
		t.Errorf("expected player_games_total{player=Alice,mode=local} = 1, got %f", got)
	}
	if got := testutil.ToFloat64(playerGamesTotal.WithLabelValues("Bob", "local")); got != 1 {
		t.Errorf("expected player_games_total{player=Bob,mode=local} = 1, got %f", got)
	}
}

func TestGameHandler_Tie(t *testing.T) {
	resetMetrics()

	game := GameResult{
		Player1: "Alice",
		Player2: "Bob",
		IsTie:   true,
		Mode:    "local",
	}
	body, _ := json.Marshal(game)

	req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gameHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if got := testutil.ToFloat64(gamesTotal.WithLabelValues("tie", "local")); got != 1 {
		t.Errorf("expected games_total{result=tie,mode=local} = 1, got %f", got)
	}
	if got := testutil.ToFloat64(tiesTotal.WithLabelValues("local")); got != 1 {
		t.Errorf("expected ties_total{mode=local} = 1, got %f", got)
	}
}

func TestGameHandler_WinStreak(t *testing.T) {
	resetMetrics()

	// Alice wins 3 times
	for i := 0; i < 3; i++ {
		game := GameResult{Player1: "Alice", Player2: "Bob", Winner: "Alice", Pattern: "row1", IsTie: false, Mode: "local"}
		body, _ := json.Marshal(game)
		req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
		w := httptest.NewRecorder()
		gameHandler(w, req)
	}

	if got := testutil.ToFloat64(winStreakGauge.WithLabelValues("Alice")); got != 3 {
		t.Errorf("expected win_streak{player=Alice} = 3, got %f", got)
	}
	if got := testutil.ToFloat64(winStreakGauge.WithLabelValues("Bob")); got != 0 {
		t.Errorf("expected win_streak{player=Bob} = 0, got %f", got)
	}
}

func TestGameHandler_StreakResetOnLoss(t *testing.T) {
	resetMetrics()

	// Alice wins
	game := GameResult{Player1: "Alice", Player2: "Bob", Winner: "Alice", Pattern: "row1", IsTie: false, Mode: "local"}
	body, _ := json.Marshal(game)
	req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
	gameHandler(httptest.NewRecorder(), req)

	// Bob wins - Alice streak resets
	game = GameResult{Player1: "Alice", Player2: "Bob", Winner: "Bob", Pattern: "col1", IsTie: false, Mode: "local"}
	body, _ = json.Marshal(game)
	req = httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
	gameHandler(httptest.NewRecorder(), req)

	if got := testutil.ToFloat64(winStreakGauge.WithLabelValues("Alice")); got != 0 {
		t.Errorf("expected Alice streak reset to 0, got %f", got)
	}
	if got := testutil.ToFloat64(winStreakGauge.WithLabelValues("Bob")); got != 1 {
		t.Errorf("expected Bob streak = 1, got %f", got)
	}
}

func TestGameHandler_InvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/game", nil)
	w := httptest.NewRecorder()
	gameHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestGameHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/game", strings.NewReader("invalid"))
	w := httptest.NewRecorder()
	gameHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got '%s'", w.Body.String())
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test OPTIONS preflight
	req := httptest.NewRequest(http.MethodOptions, "/api/game", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", w.Code)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	resetMetrics()

	handler := metricsMiddleware("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/test", "OK")); got != 1 {
		t.Errorf("expected http_requests_total = 1, got %f", got)
	}
}

func TestAllWinningPatterns(t *testing.T) {
	patterns := []string{"row1", "row2", "row3", "col1", "col2", "col3", "diag1", "diag2"}

	for _, pattern := range patterns {
		resetMetrics()
		game := GameResult{Player1: "A", Player2: "B", Winner: "A", Pattern: pattern, IsTie: false, Mode: "local"}
		body, _ := json.Marshal(game)
		req := httptest.NewRequest(http.MethodPost, "/api/game", bytes.NewReader(body))
		w := httptest.NewRecorder()
		gameHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("pattern %s: expected status 200, got %d", pattern, w.Code)
		}
		if got := testutil.ToFloat64(winsTotal.WithLabelValues("A", pattern, "local")); got != 1 {
			t.Errorf("pattern %s: expected wins_total = 1, got %f", pattern, got)
		}
	}
}
